
# Minimum Programming Language
**Minimum** is a simple scripting language written in Go. It features dynamic typing, garbage collection and 12 built-in data types. The project is aiming to provide a language similar to Python, with the benefits of easy parallel processing and cross-platform usage that come with Go.

## Documentation
### Data Types
- **Nothing**, the default data type that all functions without a return statement give back, as well as the return value within pool when an error occurs (yet to be fully implemented)
- **Int**, based on Go's `*big.Int`, infinitely big interger data type
- **Float**, infinite precision floating point number, based on `*big.Float`
- **Str**, string data type encoded with utf-8 where elements are length 1 strings
- **Arr**, array data type, to be replaced functionally by the Span data type
- **List**, list-like data type that stores references to variables within itself
- **Pair**, also known as "pairing", dict-like data type based on Go maps that may hold values of different types
- **Bool**, the regular Boolean logic data type, either `true` or `false`
- **Byte**, alias for Go `byte` data type
- **Func**, the function data type, can be called via the `!function arg0, arg1` syntax
- **Id**, also referred to as "reference", pointer-like data type that may refer to a value of any other data type within the current scope
- **Span**, the array-like data type that stores a set number of elements of the same type, returned by the `range` function

### Syntax
Indentation is based on the space character count. Usually keywords follow the syntax below:
```
keyword args:
    ...
```
List of recognized keywords in Minimum: `repeat`, `for`, `while`, `if`, `else`, `switch`, `case`, `error`, `pool`.
The dollar sign is used as a system command call character that uses the same formatting pattern as the `fmt` function.

The error statement is special and allows ignoring, handling or discriminating errors. At each level there is either zero, one, or two provided variables in the statement header.

```
error failed, info:
    !len 1
if failed:
    !print info
    #line above shows the following:
    #{"line": 2, "source": "!len 1", "action": "error", "type": "arg_type", "message": "argument 0 (_temp_0) is int, must be one of: ["str", "list", "span"]!"}

```
The first variable, if provided, is a bool that is true in case an error occurs within the handled block. The second variable can be omitted, otherwise it gets filled with a pairing containing various error data.

### Operators
The list of operators: `+`, `-`, `*`, `/`, `//`, `%`, `^` (power operator), `'` (index operator), `.` (object-like index operator), `and`, `or`
### Built-in Functions
This section will cover the most notable functions of Minimum. Here's a basic example of a function:
```
func sqrt x:
    return x^0.5
!print !sqrt 4
```
Functions create a local variable space upon being run, copying values from the outer scope. Upon finishing, the inner scope values are destroyed.
The list of functions:
- `fmt`, a function that replicates f-strings in Python, replaces expressions or variables within curly braces with their values represented by a string
- `ternary`, a simple function that accepts a bool and two values of any type; if the bool is true, the function returns the second value, otherwise - the third value is returned
- `convert`, universal conversion function that accepts two inputs: a value to convert and an example, the example's value is irrelevant to the operation
## Interpreter Usage
The interpreter is a single binary, which can be built as `minimum.exe` on Windows using the `go build -o minimum.exe -ldflags='-w -s' .` command. Whenever launched, Minimum searches the launch arguments to contain a valid file path. If found, the program reads it as a utf-8 encoded text file. It is recommended to use the `.min` extension for Minimum scripts. Minimum only accepts spaces for indentation.
Special flags:
- `-debug`, prints the program's bytecode along with execution time

## Examples
FizzBuzz:
```
n = 0
repeat 100:
    n++
    m = ""
    if n%3==0:
        m = m + "Fizz"
    if n%5==0:
        m = m + "Buzz"
    if m=="":
        !print n
    else:
        !print m
```
Cross-platform Minimum builder script, requires Go compiler:
```
platforms = ["windows", "linux"]
arches = ["amd64", "arm64"]
extra = "-ldflags='-w -s'"

!print platforms, arches
for platforms->plat:
    for arches->arch:
        name = "min-"+plat+"-"+arch
        !env "GOARCH", arch
        !env "GOOS", plat
        if plat=="windows":
            name = name+".exe"
        !print "Building:", name
        $go build -o {name} {extra} .
```
Parallel processing example with the `pool` keyword:
```
l = !convert (!range 100), []

!print l # original list
pool l->n, verbose<-m:
    m = n*n
    if m>50:
        m = "big"

!print verbose # processed list
```