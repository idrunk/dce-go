中文 | [English](README.md)

DCE-GO是一个通用路由库，除了能路由http协议外，还能路由cli、websocket、tcp/udp等非标准可路由协议的协议。

DCE-GO按功能类型不同，划分了[路由器](router)、[可路由协议](proto)、[转换器](converter)、[会话管理器](session)及[工具](util)等模块。

路由器模块下，定义了api、上下文及路由器库，以及转换器、可路由协议等接口，它是DCE的核心模块。

可路由协议模块实现了一些常见协议的可路由协议封装，包括http、cli、websocket、tcp、udp、quic等。

转换器内置实现了json与template转换器，用于序列化及反序列化串行数据、双向转换传输对象与实体对象等。

会话管理器模块定义了基础会话、用户会话、连接会话及自重生会话接口，并内置了Redis、共享内存版这些接口的实现类库。

所有功能特性，都有相应用例，位于[_examples](_examples)目录下。它的路由性能非常高，与gin相当，可在[这里](_examples/attachs/report/ab-test-result.txt)查看ab测试报告，端口`2046`的为DCE结果。

DCE-GO源自于[DCE-RUST](https://github.com/idrunk/dce-rust)，它们都源自于[DCE-PHP](https://github.com/idrunk/dce-php)。DCE-PHP是一个完整的网络编程框架，已停止更新，它的核心路由模块，已升级并迁移到DCE-RUST与DCE-GO中，目前DCE-GO功能版本较新，后续会同步两个语言的功能版本。

DCE致力于成为一个高效、开放、安全的通用路由库，欢迎你来贡献，使其更加高效、开放、安全。

TODO：
- 优化JS版Websocket可路由协议客户端，完善各协议通信GOLANG版客户端
- 升级控制器前后置事件接口，可与程序接口绑定
- 完善数字路径支持
- 尝试调整弹性数字函数，改为结构方法式
- 研究可路由协议中支持自定义业务属性的可能性
- 同步DCE-RUST
- 逐步替换AI生成的文档为人工文档
