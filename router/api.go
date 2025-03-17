package router

import (
	"log"
	"strings"

	"go.drunkce.com/dce/util"
)

const (
	MarkPathPartSeparator     = "/"
	MarkSuffixSeparator       = "|"
	MarkSuffixBoundary        = "."
	MarkVariableOpener        = "{"
	MarkVariableClosing       = "}"
	MarkVarTypeOptional       = "?"
	MarkVarTypeEmptableVector = "*"
	MarkVarTypeVector         = "+"
)

const (
	VarTypeNotVar = 1 << iota >> 1
	VarTypeRequired
	VarTypeOptional
	VarTypeVector
	VarTypeEmptableVector
)

type RpApi[Rp RoutableProtocol] struct {
	Controller func(c *Context[Rp])
	Api
}

func NewRpApi[Rp RoutableProtocol](api Api, controller func(c *Context[Rp])) *RpApi[Rp] {
	if len(api.Suffixes) > 0 {
		panic(`Please define the suffixes in the end of "Path" but not defined directly`)
	}
	lastPartFrom := strings.LastIndex(api.Path, MarkPathPartSeparator)
	if lastPartFrom == -1 {
		lastPartFrom = -len(MarkPathPartSeparator)
	}
	lastPartFrom += len(MarkPathPartSeparator)
	if boundIndex := strings.Index(api.Path[lastPartFrom:], MarkSuffixBoundary); boundIndex != -1 {
		api.Suffixes = util.MapSeqFrom[string, Suffix](strings.Split(api.Path[lastPartFrom+boundIndex+len(MarkSuffixBoundary):], MarkSuffixSeparator)).Map(func(v string) Suffix {
			return Suffix(v)
		}).Collect()
		api.Path = api.Path[:lastPartFrom+boundIndex]
	} else {
		api.Suffixes = []Suffix{""}
	}
	if controller == nil {
		controller = func(c *Context[Rp]) {}
	}
	return &RpApi[Rp]{Controller: controller, Api: api}
}

// Api represents a structured definition of an API endpoint. It encapsulates various properties
// such as the method, path, suffixes, and additional metadata like redirection, naming,
// and custom extras. The struct is designed to be flexible and extensible, allowing for the
// addition of custom key-value pairs and slice-based data through the `Extras` field.
//
// Fields:
//   - Method: Generally used to specify HTTP methods, but can also be used for other routable protocols.
//   - Path: The request path of the API endpoint, which may include dynamic parts or variables.
//   - Suffixes: A list of suffixes that can be appended to the path, typically used for versioning
//               or content negotiation.
//   - Id: A unique identifier for the API endpoint. (Not yet applied)
//   - Omission: A boolean flag indicating whether the endpoint should be omitted from request Path.
//   - Responsive: A boolean flag indicating whether the endpoint is responsive or not.
//   - Redirect: A URL to which requests to this endpoint should be redirected.
//   - Name: A human-readable name for the API endpoint.
//   - Extras: A map of additional key-value pairs that can be used to store custom data or
//             metadata related to the API endpoint.
type Api struct {
	Method     Method
	Path       string
	Suffixes   []Suffix
	Id         string
	Omission   bool
	Responsive bool
	Redirect   string
	Name       string
	extras     map[string]any
}

func (a Api) ByMethod(method Method) Api {
	a.Method = method
	return a
}

func (a Api) AsOmission() Api {
	a.Omission = true
	return a
}

func (a Api) AsResponsive() Api {
	a.Responsive = true
	return a
}

func (a Api) AsUnresponsive() Api {
	a.Responsive = false
	return a
}

func (a Api) ByRedirect(redirect string) Api {
	a.Redirect = redirect
	return a
}

func (a Api) ByName(name string) Api {
	a.Name = name
	return a
}

// With adds or updates a key-value pair in the `Extras` map of the `Api` struct. 
// If the `Extras` map is nil, it initializes it before adding the key-value pair.
// This method is useful for attaching additional metadata or custom data to the API endpoint.
//
// Parameters:
//   - key: The key under which the value will be stored in the `Extras` map.
//   - val: The value to be associated with the specified key.
//
// Returns:
//   - The modified `Api` instance with the updated `Extras` map.
func (a Api) With(key string, val any) Api {
	if a.extras == nil {
		a.extras = make(map[string]interface{})
	}
	a.extras[key] = val
	return a
}

// Append adds one or more items to a slice associated with the specified key in the `Extras` map of the `Api` struct.
// If the key does not exist in the `Extras` map, it initializes the key with a new slice containing the provided items.
// If the key exists but the associated value is not a slice, the function will panic, as it expects the value to be a slice.
// This method is useful for appending additional data to a slice stored in the `Extras` map, such as adding multiple hosts or other slice-based metadata.
//
// Parameters:
//   - key: The key in the `Extras` map to which the items will be appended.
//   - items: One or more items to append to the slice associated with the key.
//
// Returns:
//   - The modified `Api` instance with the updated `Extras` map.
func (a Api) Append(key string, items ...any) Api {
	if val := a.extras[key]; val == nil {
		if a.extras == nil {
			a.extras = make(map[string]interface{})
		}
		a.extras[key] = new([]any)
	}
	val := a.extras[key]
	if vec, ok := val.([]any); ok || len(vec) == 0 {
		a.extras[key] = append(vec, items...)
	} else {
		log.Panicf("Api with path \"%s\" was already has an extra keyd by \"%s\", but is not a slice value.", a.Path, key)
	}
	return a
}

func (a Api) ExtraBy(key string) any {
	if val, ok := a.extras[key]; ok {
		return val
	}
	return nil
}

func (a Api) ExtrasBy(key string) []any {
	if val, ok := a.extras[key]; ok {
		if vec, ok := val.([]any); ok {
			return vec
		}
	}
	return nil
}

// BindHosts appends one or more host addresses to the `Extras` map of the `Api` struct under the key `extraServeAddrKey`.
// This method is used to specify the host addresses to which the API endpoint should be bound. The host addresses are stored
// as a slice in the `Extras` map, allowing for multiple hosts to be associated with the same API endpoint.
//
// Parameters:
//   - hosts: One or more host addresses to be appended to the list of bound hosts for the API endpoint.
//
// Returns:
//   - The modified `Api` instance with the updated `Extras` map containing the appended host addresses.
func (a Api) BindHosts(hosts ...string) Api {
	return a.Append(extraServeAddrKey, util.MapSeqFrom[string, any](hosts).Map(func(v string) any {
		return any(v)
	}).Collect()...)
}

func (a Api) Hosts() []string {
	return util.MapSeqFrom[any, string](a.ExtrasBy(extraServeAddrKey)).Map(func(v any) string {
		return v.(string)
	}).Collect()
}

func Path(path string) Api {
	return Api{Path: path, Responsive: true}
}

const extraServeAddrKey = "$#BIND-HOSTS#"

type Suffix string

type Method uint
