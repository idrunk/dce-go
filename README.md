[中文](README-zh.md) | English

DCE-GO is a universal routing library that can route not only HTTP protocols but also non-standard routable protocols such as CLI, WebSocket, TCP/UDP, and more.

DCE-GO is divided into several modules based on functional types, including [Router](router), [Routable Protocols](proto), [Converters](converter), [Session Managers](session), and [Utilities](util).

The Router module defines APIs, contexts, and router libraries, as well as interfaces for converters and routable protocols. It is the core module of DCE.

The Routable Protocols module implements routable protocol encapsulations for common protocols, including HTTP, CLI, WebSocket, TCP, UDP, QUIC, and more.

The Converter module includes built-in implementations of JSON and template converters, used for serializing and deserializing data, as well as bidirectional conversion between transport objects and entity objects.

The Session Manager module defines interfaces for basic sessions, user sessions, connection sessions, and self-regenerating sessions. It also includes built-in implementations of these interfaces using Redis and shared memory.

All features come with corresponding examples, located in the [_examples](_examples) directory. Its routing performance is very high, comparable to Gin. You can view the ab test report [here](_examples/attachs/report/ab-test-result.txt). The result for port `2046` is from DCE.

DCE-GO originates from [DCE-RUST](https://github.com/idrunk/dce-rust), and both originate from [DCE-PHP](https://github.com/idrunk/dce-php). DCE-PHP was a complete network programming framework that has been discontinued. Its core routing module has been upgraded and migrated to DCE-RUST and DCE-GO. Currently, DCE-GO has a more advanced feature set, and future updates will synchronize the features between the two languages.

DCE aims to be an efficient, open, and secure universal routing library. Contributions are welcome to make it even more efficient, open, and secure.

TODO:
- Optimize the JS version of the WebSocket routable protocol client and improve the communication clients for various protocols in the Golang version.
- Upgrade the pre- and post-event interfaces of the controller to bind with program interfaces.
- Improve support for numeric paths.
- Attempt to refactor the elastic numeric function into a structural method style.
- Explore the possibility of supporting custom business attributes in routable protocols.
- Synchronize to DCE-RUST.
- Gradually replace AI-generated documentation with human-written documentation.
