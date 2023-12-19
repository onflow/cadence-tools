import "Foo"

access(all)
contract Zoo {

     access(all)
     let x: String

     init() {
         self.x = Foo.bar
     }
}