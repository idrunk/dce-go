package session

const (
	DefaultServerField = "$server"
	DefaultClientField = "$client"
)

// ConnectionSession represents a session associated with a connection. It extends the BasicSession
// and implements the IfConnection interface. This struct is used to manage session information
// for both server and client connections, including handling session updates, disconnections,
// and cloning for request-specific sessions.
type ConnectionSession struct {
	*BasicSession
	IfConnection
	// shadow is a connection level session used to update the connection session through this attribute when the sid
	// is updated under the request session, in order to update session information when the connection is disconnected
	shadow      *ConnectionSession
	request          bool
	serverField string
	clientField string
	// log the connection info to let the request session to store it
	serverToBind string
	clientToBind string
}

func NewConnectionSession(basic *BasicSession) *ConnectionSession {
	return &ConnectionSession{BasicSession: basic, serverField: DefaultServerField, clientField: DefaultClientField}
}

func (c *ConnectionSession) CloneConnection(cloned IfConnection, basic *BasicSession) *ConnectionSession {
	connection := *c
	connection.IfConnection = cloned
	connection.BasicSession = basic
	return &connection
}

// Connect bind the addresses of server and client to the session.
func (c *ConnectionSession) Connect(server string, client string) *ConnectionSession {
	c.serverToBind = server
	c.clientToBind = client
	return c
}

func (c *ConnectionSession) Disconnect() error {
	if err := c.SilentDel(c.serverField); err != nil {
		return err
	} else if err = c.SilentDel(c.clientField); err != nil {
		return err
	}
	return nil
}

func (c *ConnectionSession) Request() bool {
	return c.request
}

func (c *ConnectionSession) UpdateShadow(newSid string) error {
	return c.ReMeta(newSid)
}

func (c *ConnectionSession) CloneForRequest(sid string) (any, error) {
	cl, err := c.Clone(sid)
	if err != nil {
		return nil, err
	}
	cloned := cl.(IfConnection)
	cloned.handleClonedRequest(c)
	return cloned, nil
}

func (c *ConnectionSession) handleClonedRequest(original *ConnectionSession) {
	c.newborn = false
	c.shadow = original
	c.request = true
	if len(original.serverToBind) > 0 {
		c.serverToBind, c.clientToBind = original.serverToBind, original.clientToBind
		if err := c.SilentSet(c.serverField, c.serverToBind); err == nil {
			c.SilentSet(c.clientField, c.clientToBind)
			original.serverToBind, original.clientToBind = "", ""
		}
	}
}

type IfConnection interface {
	handleClonedRequest(original *ConnectionSession)
}
