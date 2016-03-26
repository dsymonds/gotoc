gotoc
=====
This is gotoc, a protocol buffer compiler written in Go.

This is **only** the parser side; you will need a plugin to generate code.

Quick Start
-----------
```shell
go get github.com/dsymonds/gotoc
go get github.com/golang/protobuf/protoc-gen-go
gotoc foo.proto
```

License
-------
This is licensed under the [BSD 3-Clause Licence](http://opensource.org/licenses/BSD-3-Clause).
See the LICENSE file for more details.

Read more
---------
* [The protocol buffers open source project](https://developers.google.com/protocol-buffers/)
* [The Go protocol buffer code generator plugin and support library](https://github.com/golang/protobuf)
