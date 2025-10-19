# Minimum Programming Language
**Minimum** is a simple scripting language written in Go. It features dynaming typing and 12 built-in data types. The project is aiming to provide a language similar to Python, with the benefits of easy parallel processing and cross-platform usage that come with Go.

## Documentation
**TODO**

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
