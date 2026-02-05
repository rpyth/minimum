
# Minimum Programming Language
**Minimum** is a simple scripting language written in Go. It features dynamic typing, garbage collection and 12 built-in data types. The project is aiming to provide a language similar to Python, with the benefits of easy parallel processing and cross-platform usage that come with Go.
Recommended editor for Minimum is located in the `editor` folder of the repository.

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
- `print`: accepts any number of inputs of any type (`!print a, b, c`), prints them space-separated and adds a newline, returns nothing
- `out`: accepts any number of inputs of any type (`!out a, b, c`), prints them space-separated without a newline, returns nothing
- `replace`: accepts 3–4 string inputs (`!replace str, str, str[, int]`), replaces occurrences of the second string inside the first with the third optionally limited by count, returns a single str
- `source`: accepts 1 string input (`!source path`), loads a file, compiles and executes it as Minimum code, returns nothing
- `library`: accepts 1 string input (`!library path`), launches an external executable/library process for RPC use, returns nothing
- `run`: accepts 1 string input (`!run code`), compiles and executes the provided code string, returns nothing
- `runf`: accepts 1 string input (`!runf code`), executes code in isolation and returns the final expression result, returns any type
- `isdir`: accepts 1 string input (`!isdir path`), checks whether the path is a directory, returns a bool
- `abs`: accepts 1 input (`!abs value`), returns numeric absolute value for int/float or absolute filesystem path for string, returns same type as input
- `ternary`: accepts 3 inputs (`!ternary bool, a, b`), returns the second value if condition is true otherwise the third, returns any type
- `fmt`: accepts 1 string input (`!fmt str`), formats/interpolates the string using Minimum formatting rules, returns a str
- `lower`: accepts 1 string input (`!lower str`), converts all characters to lowercase, returns a str
- `upper`: accepts 1 string input (`!upper str`), converts all characters to uppercase, returns a str
- `map`: accepts a list and a function (`!map list, func`), applies the function to each element and collects the results, returns a list
- `env`: accepts 1–2 string inputs (`!env name[, value]`), gets or sets an environment variable, returns the value when reading otherwise nothing
- `html_set_inner`: accepts 2 string inputs (`!html_set_inner selector, html`), sets the inner HTML of an element in the runtime environment, returns nothing
- `convert`: accepts 2 inputs (`!convert value, typeExample`), converts the first value to the type of the second, returns the converted value
- `value`: accepts 1 id input (`!value id`), dereferences an ID and retrieves the referenced value, returns any type
- `read`: accepts 1 string input (`!read path`), reads a file as raw bytes, returns a byte span
- `write`: accepts 2 inputs (`!write path, data`), writes a string or byte span to a file, returns nothing
- `mkdir`: accepts 1 string input (`!mkdir path`), creates a directory and parents if needed, returns nothing
- `remove`: accepts 1 string input (`!remove path`), deletes a file or directory, returns nothing
- `len`: accepts 1 input (`!len value`), computes length of a string, list, or span, returns an int
- `sleep`: accepts 1 numeric input (`!sleep seconds`), pauses execution for the specified number of seconds, returns nothing
- `range`: accepts 1–3 integer inputs (`!range end` or `!range start, end[, step]`), generates a sequence of integers, returns an int span
- `span`: accepts 1 list input (`!span list`), copies list elements into contiguous memory, returns a span
- `rand`: accepts 2 numeric inputs (`!rand min, max`), generates a random number between the bounds, returns a float
- `sort`: accepts 1–2 inputs (`!sort list[, func]`), sorts elements optionally using a key function, returns a list
- `list`: accepts any number of inputs (`!list a, b, c`), constructs a list from the provided values, returns a list
- `input`: accepts 1 string input (`!input prompt`), shows a prompt and reads a line from the user, returns a str
- `exit`: accepts 0–1 integer inputs (`!exit [code]`), terminates the program with the given exit code, returns nothing
- `system`: accepts 1 string input (`!system key`), retrieves runtime information such as os, arch, version, args, cwd, funcs, or vars, returns a value depending on key
- `keys`: accepts 1 pair input (`!keys pair`), extracts all keys from a pair/dictionary, returns a list
- `chdir`: accepts 1 string input (`!chdir path`), changes the current working directory, returns nothing
- `glob`: accepts 1 string input (`!glob pattern`), returns all filesystem paths matching a glob pattern, returns a list of strings
- `rget`: accepts 1 string input (`!rget url`), performs an HTTP GET request, returns a pair containing status code and body
- `jsonp`: accepts 1 string input (`!jsonp json`), parses JSON text into a pair/dictionary structure, returns a pair
- `rpost`: accepts 2 inputs (`!rpost url, pair`), sends an HTTP POST request with JSON body, returns a pair containing status code and body
- `split`: accepts 2 string inputs (`!split str, separator`), splits a string by the separator, returns a list of strings
- `join`: accepts a list and a string (`!join list, separator`), concatenates string elements with the separator, returns a str
- `cti`: accepts 1 string input (`!cti str`), converts the first character to its Unicode integer codepoint, returns an int
- `itc`: accepts 1 integer input (`!itc int`), converts an integer codepoint to a single character string, returns a str
- `stats`: accepts 1 string input (`!stats path`), retrieves file metadata like name, size, and timestamps, returns a pair
- `id`: accepts 1–2 inputs (`!id name` or `!id value, id`), gets a variable reference ID or assigns through an ID, returns an id or nothing
- `append`: accepts 2 inputs (`!append list_or_span, value`), adds an element to the end of the collection, returns a new list or span
- `has`: accepts 2 inputs (`!has collection, value`), checks whether the value exists inside a string, list, or span, returns a bool
- `where`: accepts 2 inputs (`!where collection, value`), finds the index of the first matching value or substring, returns an int
- `check_type`: accepts 2 inputs (`!check_type value, str`), verifies the value matches the provided type name and raises an error if not, returns nothing
- `type`: accepts 1 input (`!type value`), returns the type name of the value as text, returns a str

## Interpreter Usage
The interpreter is a single binary, which can be built as `minimum.exe` on Windows using the `go build -o minimum.exe -ldflags='-w -s' .` command. Whenever launched, Minimum searches the launch arguments to contain a valid file path. If found, the program reads it as a utf-8 encoded text file. It is recommended to use the `.min` extension for Minimum scripts. Minimum only accepts spaces for indentation.
Special flags:
- `-debug`, prints the program's bytecode along with execution time
- `-server 5000`, starts the Minimum server on the specified port (5000, for example), set the compile variable like ?compile=1 in order to save provided code after execution; pass code via the "code" filed in your json request
- `-safe`, prevents the program from using the `!write` function
- `-source`, shows the source code of the first file provided to the interpreter, useful for demonstration purposes

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