package router

import (
	"fmt"
	"log"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"sync"

	"go.drunkce.com/dce/util"
)

const CodeNotFound = 404

// Router is a generic struct that provides routing functionality for a given RoutableProtocol type.
// It manages API routes, handles path matching, and supports various routing features such as
// path variables, suffixes, and event handling. The Router is designed to be flexible and
// extensible, allowing for custom API matchers, event handlers, and route configurations.
//
// The Router maintains a tree structure for efficient path matching and supports features like
// route redirection, path omission, and dynamic path variables. It also provides methods to
// push new routes, set custom separators, and configure event handlers for pre- and post-controller
// execution.
//
// The Router is thread-safe and uses a mutex to ensure concurrent access to its internal state.
type Router[Rp RoutableProtocol] struct {
	pathPartSeparator string
	suffixBoundary    string
	apiBuffer         []*RpApi[Rp]
	rawOmittedPaths   []string
	idApiMapping      map[string]*RpApi[Rp]
	apisMapping       map[string][]*RpApi[Rp]
	apisTree          util.Tree[ApiBranch[Rp], string]
	apiMatcher        func(rp Rp, apis []*Api) (index int)
	beforeController  func(ctx *Context[Rp]) error
	afterController   func(ctx *Context[Rp]) error
	mu                sync.Mutex
}

func NewRouter[Rp RoutableProtocol]() *Router[Rp] {
	return &Router[Rp]{
		pathPartSeparator: MarkPathPartSeparator,
		suffixBoundary:    MarkSuffixBoundary,
		idApiMapping:      make(map[string]*RpApi[Rp]),
		apisMapping:       make(map[string][]*RpApi[Rp]),
		apisTree:          util.NewTree(newApiBranch("", make([]*RpApi[Rp], 0))),
	}
}

func (r *Router[Rp]) SetSeparator(pps string, sb string) *Router[Rp] {
	r.pathPartSeparator = pps
	r.suffixBoundary = sb
	return r
}

func (r *Router[Rp]) SetEventHandler(before func(ctx *Context[Rp]) error, after func(ctx *Context[Rp]) error) *Router[Rp] {
	r.beforeController = before
	r.afterController = after
	return r
}

func (r *Router[Rp]) SetApiMatcher(apiMatcher func(rp Rp, apis []*Api) (index int)) *Router[Rp] {
	r.apiMatcher = apiMatcher
	return r
}

// Push adds a new route to the router with the specified path and controller function.
// The path is the URL pattern that the route will match, and the controller function
// is the handler that will be executed when the route is matched. The controller
// function receives a context object that contains information about the request
// and provides methods to send a response.
//
// This method returns the router instance itself, allowing for method chaining.
// The route is added to the router's internal buffer and will be processed
// when the router is ready to build its routing tree.
func (r *Router[Rp]) Push(path string, controller func(c *Context[Rp])) *Router[Rp] {
	return r.PushApi(Api{Path: path, Responsive: true}, controller)
}

// PushApi adds a new route to the router with the specified API configuration and controller function.
// The `api` parameter defines the route's path, suffixes, and other properties, while the `controller`
// function is the handler that will be executed when the route is matched. The controller function
// receives a context object that contains information about the request and provides methods to send
// a response.
//
// This method returns the router instance itself, allowing for method chaining. The route is added
// to the router's internal buffer and will be processed when the router is ready to build its routing
// tree.
func (r *Router[Rp]) PushApi(api Api, controller func(c *Context[Rp])) *Router[Rp] {
	return r.PushConf(NewRpApi(api, controller))
}

func (r *Router[Rp]) PushConf(api *RpApi[Rp]) *Router[Rp] {
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.HasPrefix(api.Path, MarkPathPartSeparator) {
		log.Fatalf("`Api.Path` \"%s\" cannot start with \"%s\"\n", MarkPathPartSeparator, api.Path)
	}
	if api.Omission {
		r.rawOmittedPaths = append(r.rawOmittedPaths, api.Path)
	}
	if len(api.Id) > 0 {
		r.idApiMapping[api.Id] = api
	}
	r.apiBuffer = append(r.apiBuffer, api)
	return r
}

func (r *Router[Rp]) ready() {
	if !r.mu.TryLock() {
		return
	}
	defer r.mu.Unlock()
	r.buildTree()
	// build apisMapping
	for len(r.apiBuffer) > 0 {
		api := r.apiBuffer[0]
		path := r.omittedPath(api.Path)
		apis := []*RpApi[Rp]{api}
		suffixes := util.Set(api.Suffixes...)
		r.apiBuffer = r.apiBuffer[1:]
		for i := 0; i < len(r.apiBuffer); i++ {
			// collect the omitted same path into an array
			if path == r.omittedPath(r.apiBuffer[i].Path) {
				apis = append(apis, r.apiBuffer[i])
				suffixes.Append(r.apiBuffer[i].Suffixes...)
				// remove the collected item
				r.apiBuffer = slices.Delete(r.apiBuffer, i, i+1)
				i--
			}
		}
		// append suffix to path as api mapping key to grouping the apis
		for _, suffix := range suffixes {
			// insert suffix matched apis into the mapping
			r.apisMapping[r.suffixPath(path, &suffix)] = util.SeqFrom(apis).Filter(func(a *RpApi[Rp]) bool {
				return slices.Contains(api.Suffixes, suffix)
			}).Collect()
		}
	}
	r.apiBuffer = nil
	if r.apiMatcher == nil {
		r.apiMatcher = func(rp Rp, apis []*Api) (index int) {
			return rp.MatchApi(apis)
		}
	}
	if r.beforeController == nil {
		r.beforeController = func(ctx *Context[Rp]) error { return nil }
	}
	if r.afterController == nil {
		r.afterController = func(ctx *Context[Rp]) error { return nil }
	}
}

func (r *Router[Rp]) omittedPath(path string) string {
	// Path in api field should always be `MarkPathPartSeparator`
	parts := strings.Split(path, MarkPathPartSeparator)
	return strings.Join(util.NewMapSeq2[int, string, string](slices.All(parts)).Filter2(func(i int, _ string) bool {
		return !slices.Contains(r.rawOmittedPaths, strings.Join(parts[:i+1], MarkPathPartSeparator))
	}).Map2(func(_ int, p string) string {
		return p
	}).Collect(), MarkPathPartSeparator)
}

func (r *Router[Rp]) suffixPath(path string, suffix *Suffix) string {
	if suffix == nil || len(*suffix) == 0 {
		return path
	}
	return fmt.Sprintf("%s%s%s", path, MarkSuffixBoundary, *suffix)
}

func (r *Router[Rp]) buildTree() {
	// 1. make apis to ApiBranches
	apiBuffer := slices.Clone(r.apiBuffer)
	apiBranches := util.NewMapSeq[[]*RpApi[Rp], ApiBranch[Rp]](util.MapSeqFrom[string, []*RpApi[Rp]](util.MapSeqFrom[*RpApi[Rp], string](apiBuffer).Map(func(a *RpApi[Rp]) string {
		return a.Path
	}).Unique(slices.Contains[[]string])).Map(func(s string) []*RpApi[Rp] {
		var apis []*RpApi[Rp]
		for i := len(apiBuffer) - 1; i >= 0; i-- {
			if apiBuffer[i].Path == s {
				apis = append(apis, apiBuffer[i])
				// remove the appended
				apiBuffer = slices.Delete(apiBuffer, i, i+1)
				i--
			}
		}
		return apis
	}).Seq()).Map(func(apis []*RpApi[Rp]) ApiBranch[Rp] {
		return newApiBranch(apis[0].Path, apis)
	}).Collect()
	// 2. init the apisTree
	r.apisTree.Build(apiBranches, func(tree *util.Tree[ApiBranch[Rp], string], remains []ApiBranch[Rp]) {
		var fills []util.Tuple2[string, ApiBranch[Rp]]
		for _, remain := range remains {
			paths := strings.Split(remain.Path, MarkPathPartSeparator)
			for i := 0; i < len(paths)-1; i++ {
				path := strings.Join(paths[:i+1], MarkPathPartSeparator)
				if _, ok := tree.ChildByPath(paths[:i+1]); !ok && !util.MapSeqFrom[util.Tuple2[string, ApiBranch[Rp]], string](fills).Map(func(f util.Tuple2[string, ApiBranch[Rp]]) string {
					return f.A
				}).Contains(path, util.Equal) {
					fills = append(fills, util.NewTuple2(path, newApiBranch(path, []*RpApi[Rp]{})))
				}
			}
			// If the API already exists in `fills` and `.Apis` is empty, then need to replace with the valid API.
			if index := slices.IndexFunc(fills, func(tuple util.Tuple2[string, ApiBranch[Rp]]) bool {
				return tuple.A == remain.Path && len(tuple.B.Apis) == 0
			}); index > -1 {
				fills[index] = util.NewTuple2(remain.Path, remain)
			} else {
				// Original remain should directly insert
				fills = append(fills, util.NewTuple2(remain.Path, remain))
			}
		}
		for _, fill := range fills {
			_, _ = tree.SetByPath(strings.Split(fill.A, MarkPathPartSeparator), fill.B)
		}
	})
	// 3. fill the apisTree item properties
	r.apisTree.Traversal(func(t *util.Tree[ApiBranch[Rp], string]) int {
		isOmittedPassedChild := false
		for parent := t.Parent; parent != nil; parent = parent.Parent {
			if !parent.Element.IsOmission {
				switch parent.Element.VarType {
				case VarTypeRequired:
					parent.Element.IsMidVar = true
				case VarTypeNotVar:
					break
				default:
					panic(fmt.Sprintf("Ambiguous type var '%s' cannot in middle.", parent.Element.Key()))
				}
				if t.Element.VarType != VarTypeNotVar {
					parent.Element.VarChildren = append(parent.Element.VarChildren, t)
				} else if isOmittedPassedChild {
					parent.Element.OmittedPassedChildren[t.Element.Key()] = t
				}
				break
			}
			isOmittedPassedChild = true
		}
		return util.TreeTraverContinue
	})
}

func (r *Router[Rp]) locate(path string, apiFinder func([]*RpApi[Rp]) (*RpApi[Rp], bool)) (*RpApi[Rp], map[string]Param, *Suffix, error) {
	var api *RpApi[Rp]
	var suffix *Suffix
	var pathParams map[string]Param
	reqPath := path
	// this loop just for the RpApi.Redirect property to redirect
	for {
		apis, ok := r.apisMapping[path]
		if !ok {
			if tmpPath, tmpPathParams, tmpSuffix, ok2 := r.matchVarPath(path); ok2 {
				apis, ok = r.apisMapping[r.suffixPath(tmpPath, tmpSuffix)]
				pathParams, suffix = tmpPathParams, tmpSuffix
			}
		}
		if ok {
			if api, ok = apiFinder(apis); ok {
				if len(api.Redirect) == 0 {
					break
				}
				path = api.Redirect
				continue
			}
		}
		if len(r.apisMapping) < 1 {
			if len(r.apiBuffer) > 0 {
				r.ready()
				return r.locate(reqPath, apiFinder)
			} else {
				panic(`locate failed, "Router.apiBuffer" is empty, you may need to call the "Router.Push()" to bind apis`)
			}
		}
		return nil, nil, nil, util.Openly(CodeNotFound, `path "%s" route failed, could not matched by Router`, path)
	}
	slog.Debug(fmt.Sprintf(`%s: path "%s" matched api "%s"`, reflect.TypeFor[Rp](), reqPath, api.Path))
	return api, pathParams, suffix, nil
}

func (r *Router[Rp]) matchVarPath(path string) (string, map[string]Param, *Suffix, bool) {
	pathParts := strings.Split(path, r.pathPartSeparator)
	loopItems := []util.Tuple2[*util.Tree[ApiBranch[Rp], string], int]{util.NewTuple2(&r.apisTree, 0)}
	pathParams := map[string]Param{}
	var targetApiBranch *util.Tree[ApiBranch[Rp], string]
	var suffix *Suffix
Outer:
	for i := 0; i >= 0; i = len(loopItems) - 1 {
		apiBranch, partNumber := loopItems[i].Values()
		loopItems = loopItems[:i]
		isLastPart := partNumber == len(pathParts)-1
		isOverflowed := partNumber >= len(pathParts)
		if isOverflowed && len(apiBranch.Element.Apis) > 0 {
			// should be finished at last request path part if not a bare tree
			targetApiBranch = apiBranch
			break
		}
		// if not overflow and request path matched, then it must be a normal path
		if !isOverflowed {
			if subApiBranch, matchedSuffix, ok := r.findConsiderSuffix(pathParts[partNumber], isLastPart, apiBranch.Children, apiBranch.Element.OmittedPassedChildren); ok {
				loopItems = append(loopItems, util.NewTuple2(subApiBranch, 1+partNumber))
				suffix = matchedSuffix
				continue
			}
		}
		insertPos := len(loopItems)
		for _, varApiBranch := range apiBranch.Element.VarChildren {
			if !varApiBranch.Element.IsMidVar {
				// just need to check is_last_part because should already handle suffix if overflowed
				// pop out the last part to clean (cut off the suffix)
				suffixTrimmer := func(pathParts []string, consumer func([]string)) {
					if len(pathParts) > 0 {
						lastPart := pathParts[len(pathParts)-1]
						pathParts = pathParts[:len(pathParts)-1]
						if tmpSuffix, ok := util.MapSeqFrom[*RpApi[Rp], Suffix](varApiBranch.Element.Apis).FlatMap(func(a *RpApi[Rp]) []Suffix {
							return a.Suffixes
						}).Find(func(s Suffix) bool {
							return strings.HasSuffix(lastPart, r.suffixBoundary+string(s))
						}); ok {
							lastPart = lastPart[:len(lastPart)-len(r.suffixBoundary)-len(tmpSuffix)]
							suffix = &tmpSuffix
						}
						pathParts = append(pathParts, lastPart)
					}
					consumer(pathParts)
				}
				// if not a middle var, then should finish var path match and collect vars and end the outer loop
				if varApiBranch.Element.VarType == VarTypeOptional && isOverflowed {
					//pathParams[varApiBranch.Element.VarName] = Param{Type: varApiBranch.Element.VarType}
				} else if slices.Contains([]int{VarTypeOptional, VarTypeRequired}, varApiBranch.Element.VarType) && isLastPart {
					suffixTrimmer(pathParts, func(pp []string) {
						pathParams[varApiBranch.Element.VarName] = NewParam(pp[partNumber], varApiBranch.Element.VarType)
					})
				} else if varApiBranch.Element.VarType == VarTypeEmptableVector && isOverflowed {
					//pathParams[varApiBranch.Element.VarName] = Param{vec: []string{}, Type: varApiBranch.Element.VarType}
				} else if slices.Contains([]int{VarTypeEmptableVector, VarTypeVector}, varApiBranch.Element.VarType) && !isOverflowed {
					suffixTrimmer(pathParts, func(pp []string) {
						pathParams[varApiBranch.Element.VarName] = NewParam(pp[partNumber:], varApiBranch.Element.VarType)
					})
				} else {
					continue
				}
				targetApiBranch = varApiBranch
				break Outer
			} else if varApiBranch.Element.VarType == VarTypeRequired {
				// if it's middle var then insert to loop queue to handle it next cycle
				pathParams[varApiBranch.Element.VarName] = NewParam(pathParts[partNumber], varApiBranch.Element.VarType)
				loopItems = slices.Insert(loopItems, insertPos, util.NewTuple2(varApiBranch, 1+partNumber))
			}
		}
	}
	if targetApiBranch == nil {
		return "", nil, nil, false
	}
	return targetApiBranch.Element.Path, pathParams, suffix, true
}

func (r *Router[Rp]) findConsiderSuffix(
	part string,
	isLastPart bool,
	children map[string]*util.Tree[ApiBranch[Rp], string],
	omittedPassedChildren map[string]*util.Tree[ApiBranch[Rp], string],
) (*util.Tree[ApiBranch[Rp], string], *Suffix, bool) {
	matches, ok := children[part]
	if !ok {
		matches, ok = omittedPassedChildren[part]
	}
	// try to trim the suffix to match if not matched directly
	if !ok && isLastPart {
		for boundary := len(part); boundary > -1; boundary = strings.LastIndex(part[:boundary], r.suffixBoundary) {
			matches, ok = children[part[:boundary]]
			if !ok {
				matches, ok = omittedPassedChildren[part[:boundary]]
			}
			if ok {
				if suffix, ok := util.MapSeqFrom[*RpApi[Rp], Suffix](matches.Element.Apis).FlatMap(func(a *RpApi[Rp]) []Suffix {
					return a.Suffixes
				}).Find(func(s Suffix) bool {
					return string(s) == part[boundary+1:]
				}); ok {
					return matches, &suffix, true
				}
			}
		}
	}
	return matches, nil, matches != nil
}

// Route processes an incoming request by matching the request path against the router's configured routes.
// It locates the appropriate API handler based on the request path, extracts path parameters and suffixes,
// and invokes the corresponding controller function. If the route is not found, it sets an error on the
// request context.
//
// The method first attempts to locate the API using the request path. If the path contains dynamic segments
// (e.g., path variables), it extracts and maps them to the corresponding parameters. If a suffix is present
// in the path, it is also extracted and used to further refine the route matching.
//
// Once the API is located, the method invokes the `beforeController` event handler (if configured), executes
// the API's controller function, and then invokes the `afterController` event handler. If any of these steps
// result in an error, the error is set on the request context.
//
// This method is thread-safe and ensures that the routing logic is executed in a consistent manner, even
// when multiple requests are processed concurrently.
func (r *Router[Rp]) Route(context *Context[Rp]) {
	api, pathParams, suffix, err := r.locate(context.Rp.Path(), func(apis []*RpApi[Rp]) (*RpApi[Rp], bool) {
		if index := r.apiMatcher(context.Rp, util.MapSeqFrom[*RpApi[Rp], *Api](apis).Map(func(a *RpApi[Rp]) *Api {
			return &a.Api
		}).Collect()); index > -1 {
			return apis[index], true
		}
		return nil, false
	})
	if err == nil {
		err = r.routedHandle(api, pathParams, suffix, context)
	}
	if err != nil {
		context.Rp.SetError(err)
	}
}

func (r *Router[Rp]) routedHandle(api *RpApi[Rp], pathParams map[string]Param, suffix *Suffix, context *Context[Rp]) error {
	context.SetRoutes(r, api, pathParams, suffix)
	if err := r.beforeController(context); err != nil {
		return err
	}
	api.Controller(context)
	return r.afterController(context)
}

func (r *Router[Rp]) idLocate(id string) (*RpApi[Rp], error) {
	if api, ok := r.idApiMapping[id]; ok {
		slog.Debug(fmt.Sprintf(`%s: Uid "%s" matched api "%s"`, reflect.TypeFor[Rp](), id, api.Path))
		return api, nil
	}
	return nil, util.Openly(CodeNotFound, `Uid "%s" route failed, could not matched by Router`, id)
}

func (r *Router[Rp]) IdRoute(context *Context[Rp]) {
	api, err := r.idLocate(context.Rp.Path())
	if err == nil {
		err = r.routedHandle(api, map[string]Param{}, nil, context)
	}
	if err != nil {
		context.Rp.SetError(err)
	}
}

type ApiBranch[Rp RoutableProtocol] struct {
	Path                  string
	VarType               int
	VarName               string
	IsMidVar              bool
	IsOmission            bool
	Apis                  []*RpApi[Rp]
	VarChildren           []*util.Tree[ApiBranch[Rp], string]
	OmittedPassedChildren map[string]*util.Tree[ApiBranch[Rp], string]
}

func (ab ApiBranch[Rp]) Key() string {
	if index := strings.LastIndex(ab.Path, MarkPathPartSeparator); index > -1 {
		return ab.Path[index+1:]
	}
	return ab.Path
}

func (ab ApiBranch[Rp]) ChildOf(parent any) bool {
	if index := strings.LastIndex(ab.Path, MarkPathPartSeparator); index > -1 {
		return ab.Path[:index] == parent.(ApiBranch[Rp]).Path
	}
	return len(parent.(ApiBranch[Rp]).Path) < 1
}

func (ab ApiBranch[Rp]) EqualTo(other any) bool {
	return ab.Path == other.(ApiBranch[Rp]).Path
}

func (ab ApiBranch[Rp]) fillVarType() ApiBranch[Rp] {
	key := ab.Key()
	if strings.HasPrefix(key, MarkVariableOpener) && strings.HasSuffix(key, MarkVariableClosing) {
		if ab.IsOmission {
			panic("Var path could not be omissible.")
		}
		varName := key[len(MarkVariableOpener) : len(key)-len(MarkVariableOpener)]
		if strings.HasSuffix(varName, MarkVarTypeOptional) {
			ab.VarType = VarTypeOptional
			ab.VarName = varName[:len(varName)-len(MarkVarTypeOptional)]
		} else if strings.HasSuffix(varName, MarkVarTypeEmptableVector) {
			ab.VarType = VarTypeEmptableVector
			ab.VarName = varName[:len(varName)-len(MarkVarTypeEmptableVector)]
		} else if strings.HasSuffix(varName, MarkVarTypeVector) {
			ab.VarType = VarTypeVector
			ab.VarName = varName[:len(varName)-len(MarkVarTypeVector)]
		} else {
			ab.VarType = VarTypeRequired
			ab.VarName = varName
		}
	}
	return ab
}

func newApiBranch[Rp RoutableProtocol](path string, apis []*RpApi[Rp]) ApiBranch[Rp] {
	return ApiBranch[Rp]{
		Path:                  path,
		Apis:                  apis,
		OmittedPassedChildren: make(map[string]*util.Tree[ApiBranch[Rp], string]),
	}.fillVarType()
}

func ProtoRouter[Rp RoutableProtocol](key string) *Router[Rp] {
	router, ok := protoRouterMap.Load(key)
	if !ok {
		router = NewRouter[Rp]()
		protoRouterMap.Store(key, router)
	}
	return router.(*Router[Rp])
}

var protoRouterMap sync.Map
