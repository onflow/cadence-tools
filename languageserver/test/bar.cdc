import Foo from "./foo.cdc"

access(all) contract Bar {
     access(all) let x: String
     init() {
       self.x = Foo.bar
     }
}