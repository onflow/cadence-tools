import "Foo"

pub contract Zoo {
     pub let x: String
     init() {
       self.x = Foo.bar
     }
}