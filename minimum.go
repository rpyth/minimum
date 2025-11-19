package main

import (
	"fmt"
	"io"
	"math/big"
	"math/rand/v2"
	"minimum/bytecode"
	"minimum/inter"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/chzyer/readline"
)

const (
	NOTH = iota
	INT
	FLOAT
	STR
	ARR
	LIST
	PAIR
	BOOL
	BYTE
	FUNC
	ID
	SPAN
)

func ternary[T any](cond bool, if_true, if_false T) T {
	if cond {
		return if_true
	} else {
		return if_false
	}
}

func Uint64() uint64 {
	return uint64(rand.Uint32())<<32 + uint64(rand.Uint32())
}

func PowInt(number, power *big.Int) *big.Int {
	result := big.NewInt(0)
	result.Set(power.Exp(number, power, nil))
	return result
}

func split_code(code string) []string {
	splitted := []string{""}
	for len(code) > 0 {
		//fmt.Println(code)
		switch {
		case strings.HasPrefix(code, "\\\""):
			splitted[len(splitted)-1] += "\""
			code = code[2:]
		case strings.HasPrefix(code, "\\\\"):
			splitted[len(splitted)-1] += "\\"
			code = code[2:]
		case strings.HasPrefix(code, "\\\\n"):
			splitted[len(splitted)-1] += "\\n"
			code = code[3:]
		case strings.HasPrefix(code, "\\n"):
			splitted[len(splitted)-1] += "\n"
			code = code[2:]
		case strings.HasPrefix(code, "\\\\r"):
			splitted[len(splitted)-1] += "\\r"
			code = code[3:]
		case strings.HasPrefix(code, "\\r"):
			splitted[len(splitted)-1] += "\r"
			code = code[2:]
		case strings.HasPrefix(code, "\""):
			splitted = append(splitted, "")
			code = code[1:]
		default:
			r, size := utf8.DecodeRuneInString(code)
			splitted[len(splitted)-1] += string(r) //code[0:1]
			code = code[size:]
		}
	}
	return splitted
}

func remove_strings(source string) (string, map[string]string) {
	str_map := make(map[string]string)
	parts := split_code(source)
	code_parts := []string{}
	name_n := 0
	for n := 0; n < len(parts); n++ {
		if n%2 == 0 {
			code_parts = append(code_parts, parts[n])
		} else {
			name := fmt.Sprintf("_str_%d", name_n)
			name_n++
			code_parts = append(code_parts, name)
			str_map[name] = parts[n]
			//println(parts[n])
		}
	}
	return strings.Join(code_parts, "\""), str_map
}

// Taken from https://stackoverflow.com/questions/21060945/simple-way-to-copy-a-file
// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherise, attempt to create a hard link
// between the two files. If that fail, copy the file contents from src to dst.
func CopyFile(src, dst string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	if err = os.Link(src, dst); err == nil {
		return
	}
	err = copyFileContents(src, dst)
	return
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

func find_file(args []string) string {
	fname := ""
	for _, arg := range args[1:] {
		if _, err := os.Stat(arg); err == nil {
			return arg
		}
	}
	return fname
}

func get_script_path(executable string) string {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)
	absolute := filepath.Join(exPath, executable)

	if find_file([]string{"dummy", absolute}) != "" {
		return absolute
	} else {
		if runtime.GOOS == "windows" {
			absolute = ".\\" + executable
		} else {
			absolute = "./" + executable
		}
		return absolute
	}
}

func find_file_main(args []string) string {
	fname := ""
	for _, arg := range args[1:] {
		if _, err := os.Stat(arg); err == nil {
			return arg
		}
		if _, err := os.Stat(get_script_path(arg)); err == nil {
			return get_script_path(arg)
		}
	}
	return fname
}

func contains_any(where string, what []string) bool {
	for _, item := range what {
		if strings.Contains(where, item) {
			return true
		}
	}
	return false
}

var rl *readline.Instance

// read-only flags start
var is_debug bool
var is_safe bool
var error_message string
var error_type string

func init() {
	rl, _ = readline.New(">> ")
	inter.RL, _ = readline.New(">> ")
	is_debug = bytecode.Has(os.Args, "-debug")
	is_safe = bytecode.Has(os.Args, "-safe")
}

func timer(name string) func() {
	start := time.Now()
	return func() {
		fmt.Printf("%s took %v\n", name, time.Since(start))
	}
}

func main() {
	fname := find_file_main(os.Args)
	if fname != "" {
		bcode, _ := os.ReadFile(fname)
		code := string(bcode)
		if is_debug {
			defer timer("interpreter")()
		}
		in := inter.NewInterpreter(code, fname)
		if is_debug {
			bytecode.PrintActs(in.Code)
		}
		in.Nothing("Nothing")
		in.Run(fmt.Sprintf("_node_%d", bytecode.NodeN-1))
		//inter.StartFull(code, fname)
		return
	}
	// REPL START
	synt := []string{"if ", "while ", "pool ", "else", "set ", "for ", "func ", "process ", "repeat "}
	//counters
	var bracket_c, paren_c, curly_c, quote_c int
	//uncounters
	var bracket_u, paren_u, curly_u int
	rl, _ := readline.New("?>>")
	defer rl.Close()
	in := inter.NewInterpreter(`!print "[Minimum v"+(!system "version")+" on "+(!system "os")+"]"`, ".") //Interpreter{}
	in.Nothing("Nothing")
	in.Run(fmt.Sprintf("_node_%d", bytecode.NodeN-1))
	for {
		in.GC()
		source, err_ := rl.Readline()
		if err_ != nil { // io.EOF
			return
		}
		bracket_u += strings.Count(source, "]")
		paren_u += strings.Count(source, ")")
		curly_u += strings.Count(source, "}")
		bracket_c += strings.Count(source, "[")
		paren_c += strings.Count(source, "(")
		curly_c += strings.Count(source, "{")
		quote_c += strings.Count(source, "\"")
		break_entry := !contains_any(source, synt) && (bracket_c == bracket_u && paren_c == paren_u && curly_c == curly_u && quote_c%2 == 0)
		for !break_entry {
			rl.SetPrompt(" >>")
			sourcelet, err_ := rl.Readline()
			if err_ != nil { // io.EOF
				return
			}
			if strings.HasPrefix(sourcelet, " ") {
				source += "\n" + sourcelet
			} else if !(bracket_c == bracket_u && paren_c == paren_u && curly_c == curly_u && quote_c%2 == 0) {
				source += "\n" + sourcelet
			} else {
				break_entry = true
			}
			bracket_u += strings.Count(sourcelet, "]")
			paren_u += strings.Count(sourcelet, ")")
			curly_u += strings.Count(sourcelet, "}")
			bracket_c += strings.Count(sourcelet, "[")
			paren_c += strings.Count(sourcelet, "(")
			curly_c += strings.Count(sourcelet, "{")
			quote_c += strings.Count(sourcelet, "\"")
		}
		rl.SetPrompt("?>>")
		in.Compile(source, ".")
		last_node := fmt.Sprintf("_node_%d", bytecode.NodeN-1)
		if len(in.Code[last_node]) == 0 {
			continue
		}
		gc := in.Code[last_node][len(in.Code[last_node])-1]
		in.Code[last_node] = in.Code[last_node][:len(in.Code[last_node])-1]
		acts := in.Code[last_node]
		err := in.Run(last_node)
		if len(acts) == 0 {
			continue
		}
		last_name := acts[len(acts)-1].Target
		if !err && len(acts) > 0 && !bytecode.Has([]string{"print", "out", "source", "library"}, acts[len(acts)-1].Type) {
			switch in.V.Slots[in.V.Names[acts[len(acts)-1].Target]].Type {
			case INT:
				i := in.NamedInt(last_name)
				fmt.Println(i.String())
			case FLOAT:
				i := in.NamedFloat(last_name)
				fmt.Println(i.String())
			case STR:
				fmt.Println("\"" + in.NamedStr(last_name) + "\"")
			case LIST:
				i := in.NamedList(last_name)
				fmt.Println(inter.ListString(&i, &in))
			case ARR:
				l := in.NamedArr(last_name)
				fmt.Print(l.String())
			case PAIR:
				p := in.NamedPair(last_name)
				fmt.Println(inter.PairString(&p, &in))
			case BOOL:
				fmt.Println(in.NamedBool(last_name))
			case BYTE:
				fmt.Printf("b.%d\n", in.NamedByte(last_name))
			case FUNC:
				fmt.Println("func." + in.NamedFunc(last_name).Name)
			case SPAN:
				fmt.Println(in.StringSpan(in.NamedSpan(last_name)))
			case NOTH:
				fmt.Println("Nothing")
			}
		}
		in.Code[last_node] = append(in.Code[last_node], gc)
	}
	// REPL END
	/*
		fname := find_file_main(os.Args)
		if fname != "" {
			bcode, _ := os.ReadFile(fname)
			code := string(bcode)
			in := NewInterpreter(code, fname)
			if is_debug {
				defer timer("interpreter")()
				bytecode.PrintActs(in.Code)
			}
			in.Run(fmt.Sprintf("_node_%d", bytecode.NodeN-1))
		} else {
			synt := []string{"if ", "while ", "pool ", "else", "set ", "for ", "func ", "process ", "repeat "}
			//counters
			var bracket_c, paren_c, curly_c, quote_c int
			//uncounters
			var bracket_u, paren_u, curly_u int
			rl, _ := readline.New("?>>")
			defer rl.Close()
			in := NewInterpreter(`!print "[Minimum v"+(!system "version")+" on "+(!system "os")+"]"`, ".") //Interpreter{}
			in.Run(fmt.Sprintf("_node_%d", bytecode.NodeN-1))
			for true {
				source, err_ := rl.Readline()
				if err_ != nil { // io.EOF
					return
				}
				bracket_u += strings.Count(source, "]")
				paren_u += strings.Count(source, ")")
				curly_u += strings.Count(source, "}")
				bracket_c += strings.Count(source, "[")
				paren_c += strings.Count(source, "(")
				curly_c += strings.Count(source, "{")
				quote_c += strings.Count(source, "\"")
				break_entry := !contains_any(source, synt) && (bracket_c == bracket_u && paren_c == paren_u && curly_c == curly_u && quote_c%2 == 0)
				for !break_entry {
					rl.SetPrompt(" >>")
					sourcelet, err_ := rl.Readline()
					if err_ != nil { // io.EOF
						return
					}
					if strings.HasPrefix(sourcelet, " ") {
						source += "\n" + sourcelet
					} else if !(bracket_c == bracket_u && paren_c == paren_u && curly_c == curly_u && quote_c%2 == 0) {
						source += "\n" + sourcelet
					} else {
						break_entry = true
					}
					bracket_u += strings.Count(sourcelet, "]")
					paren_u += strings.Count(sourcelet, ")")
					curly_u += strings.Count(sourcelet, "}")
					bracket_c += strings.Count(sourcelet, "[")
					paren_c += strings.Count(sourcelet, "(")
					curly_c += strings.Count(sourcelet, "{")
					quote_c += strings.Count(sourcelet, "\"")
				}
				rl.SetPrompt("?>>")
				in.Compile(source, ".")
				last_node := fmt.Sprintf("_node_%d", bytecode.NodeN-1)
				gc := in.Code[last_node][len(in.Code[last_node])-1]
				in.Code[last_node] = in.Code[last_node][:len(in.Code[last_node])-1]
				acts := in.Code[last_node]
				err := in.Run(last_node)
				if !err && len(acts) > 0 && !bytecode.Has([]string{"print", "out", "source", "library"}, acts[len(acts)-1].Type) {
					switch in.V.Types[in.V.Names[acts[len(acts)-1].Target]] {
					case INT:
						i := in.V.Ints[in.V.Names[acts[len(acts)-1].Target]]
						fmt.Println(i.String())
					case FLOAT:
						i := in.V.Floats[in.V.Names[acts[len(acts)-1].Target]]
						fmt.Println(i.String())
					case STR:
						fmt.Println("\"" + in.V.Strs[in.V.Names[acts[len(acts)-1].Target]] + "\"")
					case LIST:
						i := in.V.Lists[in.V.Names[acts[len(acts)-1].Target]]
						fmt.Println(ListString(&i, &in))
					case ARR:
						l := in.V.Arrs[in.V.Names[acts[len(acts)-1].Target]]
						fmt.Print(l.String())
					case PAIR:
						p := in.V.Pairs[in.V.Names[acts[len(acts)-1].Target]]
						fmt.Println(PairString(&p, &in))
						//i := in.v.Pairs[acts[len(acts)-1].Target]
						//fmt.Println(i.String())
					case BOOL:
						fmt.Println(in.V.Bools[in.V.Names[acts[len(acts)-1].Target]])
					case BYTE:
						fmt.Printf("b.%d\n", in.V.Bytes[in.V.Names[acts[len(acts)-1].Target]])
					case FUNC:
						fmt.Println("func." + in.V.Funcs[in.V.Names[acts[len(acts)-1].Target]].Name)
						//fmt.Printf("func.%s\n", in.V.Funs[in.V.Names[acts[len(acts)-1].Target]])
					}
				}
				in.Code[last_node] = append(in.Code[last_node], gc)
			}
			return
		}
	*/
}
