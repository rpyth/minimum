package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/big"
	"math/rand/v2"
	"minimum/bytecode"
	"minimum/inter"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
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

func ArrAppend(l *bytecode.Array, in *Interpreter, item any) string {
	ref := in.GetRef(item)
	if l.Dtype == NOTH {
		l.Dtype = in.V.Types[ref]
	}
	if in.V.Types[ref] != l.Dtype {
		dstrings := map[byte]string{0: "noth", 1: "int", 2: "float", 3: "str", 4: "arr", 5: "list", 6: "pair", 7: "bool", 8: "byte", 9: "func", 10: "id"}
		return fmt.Sprintf("array item %d is of type %s, must be %s", len(l.Ids), dstrings[in.V.Types[ref]], dstrings[l.Dtype])
	}
	l.Ids = append(l.Ids, ref)
	return ""
}

func ArrString(l *bytecode.Array, in *Interpreter) string {
	elements := []string{}
	dstrings := map[byte]string{0: "noth", 1: "int", 2: "float", 3: "str", 4: "arr", 5: "list", 6: "pair", 7: "bool", 8: "byte", 9: "func", 10: "id"}
	for _, item := range l.Ids {
		switch in.V.Types[item] {
		case INT:
			elements = append(elements, in.V.Ints[item].String())
		case FLOAT:
			elements = append(elements, in.V.Floats[item].String())
		case BOOL:
			elements = append(elements, ternary(in.V.Bools[item], "true", "false"))
		case BYTE:
			elements = append(elements, fmt.Sprintf("b.%d", in.V.Bytes[item]))
		case STR:
			elements = append(elements, "\""+in.V.Strs[item]+"\"")
		case FUNC:
			elements = append(elements, "func."+in.V.Funcs[item].Name)
		case LIST:
			ll := in.V.Lists[item]
			elements = append(elements, ListString(&ll, in))
		case PAIR:
			pp := in.V.Pairs[item]
			elements = append(elements, PairString(&pp, in))
		default:
			elements = append(elements, "Nothing")
		}
	}
	return dstrings[l.Dtype] + ".[" + strings.Join(elements, ", ") + "]"
}

func ArrMAppend(l *bytecode.Span, in *Interpreter, item any) string {
	var ref uint64
	if l.Dtype == NOTH { // first item add
		ref = in.GetRef(item)
		l.Dtype = in.V.Types[ref]
		l.Start = ref
	} else {
		ref = l.Start + l.Length
		if _, ok := in.V.Types[ref]; ok {
			in.ChangeRef(ref)
		}
		in.SaveRef(ref, item)
	}
	if in.V.Types[ref] != l.Dtype {
		dstrings := map[byte]string{0: "noth", 1: "int", 2: "float", 3: "str", 4: "arr", 5: "list", 6: "pair", 7: "bool", 8: "byte", 9: "func", 10: "id"}
		return fmt.Sprintf("array item %d is of type %s, must be %s", l.Length, dstrings[in.V.Types[ref]], dstrings[l.Dtype])
	}
	// l.Ids = append(l.Ids, ref)
	l.Length++
	return ""
}

func ArrMString(l *bytecode.Span, in *Interpreter) string {
	elements := []string{}
	dstrings := map[byte]string{0: "noth", 1: "int", 2: "float", 3: "str", 4: "arr", 5: "list", 6: "pair", 7: "bool", 8: "byte", 9: "func", 10: "id", 11: "arrm"}
	for item := l.Start; item < l.Start+l.Length; item++ {
		switch l.Dtype {
		case INT:
			elements = append(elements, in.V.Ints[item].String())
		case FLOAT:
			elements = append(elements, in.V.Floats[item].String())
		case BOOL:
			elements = append(elements, ternary(in.V.Bools[item], "true", "false"))
		case BYTE:
			elements = append(elements, fmt.Sprintf("b.%d", in.V.Bytes[item]))
		case STR:
			elements = append(elements, "\""+in.V.Strs[item]+"\"")
		case FUNC:
			elements = append(elements, "func."+in.V.Funcs[item].Name)
		case LIST:
			ll := in.V.Lists[item]
			elements = append(elements, ListString(&ll, in))
		case PAIR:
			pp := in.V.Pairs[item]
			elements = append(elements, PairString(&pp, in))
		default:
			elements = append(elements, "Nothing")
		}
	}
	return dstrings[l.Dtype] + ".[" + strings.Join(elements, ", ") + "]"
}

func ListAppend(l *bytecode.List, in *Interpreter, item any) {
	ref := in.GetRef(item)
	l.Ids = append(l.Ids, ref)
}

func ListString(l *bytecode.List, in *Interpreter) string {
	elements := []string{}
	for _, item := range l.Ids {
		switch in.V.Types[item] {
		case INT:
			elements = append(elements, in.V.Ints[item].String())
		case FLOAT:
			elements = append(elements, in.V.Floats[item].String())
		case BOOL:
			elements = append(elements, ternary(in.V.Bools[item], "true", "false"))
		case BYTE:
			elements = append(elements, fmt.Sprintf("b.%d", in.V.Bytes[item]))
		case STR:
			elements = append(elements, "\""+in.V.Strs[item]+"\"")
		case FUNC:
			elements = append(elements, "func."+in.V.Funcs[item].Name)
		case LIST:
			ll := in.V.Lists[item]
			elements = append(elements, ListString(&ll, in))
		case PAIR:
			pp := in.V.Pairs[item]
			elements = append(elements, PairString(&pp, in))
		default:
			elements = append(elements, "Nothing")
		}
	}
	return "[" + strings.Join(elements, ", ") + "]"
}

func PairString(p *bytecode.Pair, in *Interpreter) string {
	elements := []string{}
	for key, item := range p.Ids {
		i := 0
		for key[i] != ':' {
			i++
		}
		dkey := key[i+1:]
		if key[:i] == "str" {
			dkey = "\"" + dkey + "\""
		}
		switch in.V.Types[item] {
		case INT:
			elements = append(elements, dkey+": "+in.V.Ints[item].String())
		case FLOAT:
			elements = append(elements, dkey+": "+in.V.Floats[item].String())
		case BOOL:
			elements = append(elements, dkey+": "+ternary(in.V.Bools[item], "true", "false"))
		case BYTE:
			elements = append(elements, dkey+": "+fmt.Sprintf("b.%d", in.V.Bytes[item]))
		case STR:
			elements = append(elements, dkey+": \""+in.V.Strs[item]+"\"")
		case LIST:
			ll := in.V.Lists[item]
			elements = append(elements, dkey+": "+ListString(&ll, in))
		case FUNC:
			elements = append(elements, dkey+": "+"func."+in.V.Funcs[item].Name)
		case PAIR:
			pp := in.V.Pairs[item]
			elements = append(elements, dkey+": "+PairString(&pp, in))
		}
	}
	return "{" + strings.Join(elements, ", ") + "}"
}

func ListUnlink(l *bytecode.List, in *Interpreter) {
	for n, id := range l.Ids {
		switch in.V.Types[id] {
		case INT:
			new_id := in.GetRef(in.V.Ints[id])
			l.Ids[n] = new_id
		case FLOAT:
			new_id := in.GetRef(in.V.Floats[id])
			l.Ids[n] = new_id
		case STR:
			new_id := in.GetRef(in.V.Strs[id])
			l.Ids[n] = new_id
		case LIST:
			new_id := in.GetRef(in.V.Lists[id])
			l.Ids[n] = new_id
		case ARR:
			new_id := in.GetRef(in.V.Arrs[id])
			l.Ids[n] = new_id
		case PAIR:
			new_id := in.GetRef(in.V.Pairs[id])
			l.Ids[n] = new_id
		case BOOL:
			new_id := in.GetRef(in.V.Bools[id])
			l.Ids[n] = new_id
		case BYTE:
			new_id := in.GetRef(in.V.Bytes[id])
			l.Ids[n] = new_id
		case ID:
			new_id := in.GetRef(in.V.Ids[id])
			l.Ids[n] = new_id
		case FUNC:
			new_id := in.GetRef(in.V.Funcs[id])
			l.Ids[n] = new_id
		}
	}
}

func PairKey(in *Interpreter, iname any) string {
	key_ref := in.GetRef(iname)
	dtype := map[byte]string{0: "noth", 1: "int", 2: "float", 3: "str", 4: "arr", 5: "list", 6: "pair", 7: "bool", 8: "byte", 9: "func", 10: "id"}
	var iname_str string
	switch in.V.Types[key_ref] { //TODO: add all types
	case INT:
		iname_str = in.V.Ints[key_ref].String()
	case FLOAT:
		iname_str = in.V.Floats[key_ref].String()
	case BYTE:
		iname_str = fmt.Sprintf("b.%d", in.V.Bytes[key_ref])
	case BOOL:
		iname_str = ternary(in.V.Bools[key_ref], "true", "false")
	case FUNC:
		iname_str = fmt.Sprintf("func.%s", in.V.Funcs[key_ref].Name)
	case LIST:
		l := in.V.Lists[key_ref]
		iname_str = ListString(&l, in)
	case PAIR:
		p := in.V.Pairs[key_ref]
		iname_str = PairString(&p, in)
	case STR:
		iname_str = in.V.Strs[key_ref]
	}
	key := fmt.Sprintf("%s:%s", dtype[in.V.Types[key_ref]], iname_str)
	return key
}

func PairAppend(p *bytecode.Pair, in *Interpreter, item any, iname any) {
	value_ref := in.GetRef(item)
	key_ref := in.GetRef(iname)
	dtype := map[byte]string{0: "noth", 1: "int", 2: "float", 3: "str", 4: "arr", 5: "list", 6: "pair", 7: "bool", 8: "byte", 9: "func", 10: "id"}
	var iname_str string
	switch in.V.Types[key_ref] { //TODO: add all types
	case INT:
		iname_str = in.V.Ints[key_ref].String()
	case FLOAT:
		iname_str = in.V.Floats[key_ref].String()
	case BYTE:
		iname_str = fmt.Sprintf("b.%d", in.V.Bytes[key_ref])
	case BOOL:
		iname_str = ternary(in.V.Bools[key_ref], "true", "false")
	case FUNC:
		iname_str = fmt.Sprintf("func.%s", in.V.Funcs[key_ref].Name)
	case LIST:
		l := in.V.Lists[key_ref]
		iname_str = ListString(&l, in)
	case PAIR:
		p := in.V.Pairs[key_ref]
		iname_str = PairString(&p, in)
	case ARR:
		a := in.V.Arrs[key_ref]
		iname_str = a.String()
	case STR:
		iname_str = in.V.Strs[key_ref]
	}
	key := fmt.Sprintf("%s:%s", dtype[in.V.Types[key_ref]], iname_str)
	p.Ids[key] = value_ref
}

func PairGetRef(p *bytecode.Pair, in *Interpreter, key any) uint64 {
	var full_key string
	switch val := key.(type) { //TODO: add all types
	case string:
		full_key = "str:" + val
	case *big.Int:
		full_key = "int:" + val.String()
	}
	return p.Ids[full_key]
}

type Interpreter struct {
	V         *bytecode.Vars
	Code      map[string][]bytecode.Action
	Parent    *Interpreter
	File      *string
	halt      bool
	SwitchId  string
	IgnoreErr bool
	ErrSource *bytecode.SourceLine
}

func Uint64() uint64 {
	return uint64(rand.Uint32())<<32 + uint64(rand.Uint32())
}

func (in *Interpreter) Destroy() {
	for key := range in.V.Names {
		in.RemoveName(key)
	}

	in.GC2()

	/*
		for key := range in.Code {
			delete(in.Code, key)
		}
	*/

	in = &Interpreter{}
}

func (in *Interpreter) ChangeRef(og_ref uint64) uint64 {
	// TODO: check for deep references
	id := og_ref
	ok := true
	for ok {
		id = Uint64()
		_, ok = in.V.Types[id]
	}
	copy := in.GetAnyRef(og_ref)
	in.SaveRef(id, copy)
	in.RemoveId(og_ref)
	return id
}

func (in *Interpreter) Save(name string, v any) {
	id := uint64(0)
	ok := false
	if id, ok = in.V.Names[name]; ok {
		in.RemoveId(id)
	} else {
		ok = true
		for ok {
			id = Uint64()
			_, ok = in.V.Types[id]
		}
	}
	delete(in.V.Types, id)
	switch val := v.(type) {
	case *big.Int:
		in.V.Ints[id] = val
		in.V.Names[name] = id
		in.V.Types[id] = INT
	case *big.Float:
		in.V.Floats[id] = val
		in.V.Names[name] = id
		in.V.Types[id] = FLOAT
	case string:
		in.V.Strs[id] = val
		in.V.Names[name] = id
		in.V.Types[id] = STR
	case byte:
		in.V.Bytes[id] = val
		in.V.Names[name] = id
		in.V.Types[id] = BYTE
	case bool:
		in.V.Bools[id] = val
		in.V.Names[name] = id
		in.V.Types[id] = BOOL
	case uint64:
		in.V.Ids[id] = val
		in.V.Names[name] = id
		in.V.Types[id] = ID
	case bytecode.List:
		in.V.Lists[id] = val
		in.V.Names[name] = id
		in.V.Types[id] = LIST
	case bytecode.Array:
		in.V.Arrs[id] = val
		in.V.Names[name] = id
		in.V.Types[id] = ARR
	case bytecode.Pair:
		in.V.Pairs[id] = val
		in.V.Names[name] = id
		in.V.Types[id] = PAIR
	case bytecode.Function:
		in.V.Funcs[id] = val
		in.V.Names[name] = id
		in.V.Types[id] = FUNC
	case bytecode.Span:
		in.V.Spans[id] = val
		in.V.Names[name] = id
		in.V.Types[id] = SPAN
	}
}

func (in *Interpreter) SaveRef(id uint64, v any) {
	switch val := v.(type) {
	case *big.Int:
		in.V.Ints[id] = val
		in.V.Types[id] = INT
	case *big.Float:
		in.V.Floats[id] = val
		in.V.Types[id] = FLOAT
	case string:
		in.V.Strs[id] = val
		in.V.Types[id] = STR
	case byte:
		in.V.Bytes[id] = val
		in.V.Types[id] = BYTE
	case bool:
		in.V.Bools[id] = val
		in.V.Types[id] = BOOL
	case uint64:
		in.V.Ids[id] = val
		in.V.Types[id] = ID
	case bytecode.List:
		in.V.Lists[id] = val
		in.V.Types[id] = LIST
	case bytecode.Array:
		in.V.Arrs[id] = val
		in.V.Types[id] = ARR
	case bytecode.Pair:
		in.V.Pairs[id] = val
		in.V.Types[id] = PAIR
	case bytecode.Function:
		in.V.Funcs[id] = val
		in.V.Types[id] = FUNC
	}
}

func (in *Interpreter) GetRef(v any) uint64 {
	id := uint64(0)
	ok := true
	for ok {
		id = Uint64()
		_, ok = in.V.Types[id]
	}
	switch val := v.(type) {
	case *big.Int:
		in.V.Ints[id] = val
		in.V.Types[id] = INT
	case *big.Float:
		in.V.Floats[id] = val
		in.V.Types[id] = FLOAT
	case string:
		in.V.Strs[id] = val
		in.V.Types[id] = STR
	case byte:
		in.V.Bytes[id] = val
		in.V.Types[id] = BYTE
	case bool:
		in.V.Bools[id] = val
		in.V.Types[id] = BOOL
	case bytecode.List:
		in.V.Lists[id] = val
		in.V.Types[id] = LIST
	case bytecode.Array:
		in.V.Arrs[id] = val
		in.V.Types[id] = ARR
	case bytecode.Pair:
		in.V.Pairs[id] = val
		in.V.Types[id] = PAIR
	case bytecode.Function:
		in.V.Funcs[id] = val
		in.V.Types[id] = FUNC
	}
	return id
}

func (in *Interpreter) EqualizeTypes(v1, v2 string) (string, string) { // returns tempvar names
	t1, t2 := in.V.Types[in.V.Names[v1]], in.V.Types[in.V.Names[v2]]
	if t1 == t2 {
		return v1, v2
	}
	if t1 == INT && t2 == FLOAT {
		converted := big.NewFloat(0)
		converted.SetString(in.V.Ints[in.V.Names[v1]].String())
		in.Save("_temp_a", converted)
		return "_temp_a", v2
	} else if t1 == FLOAT && t2 == INT {
		converted := big.NewFloat(0)
		converted.SetString(in.V.Ints[in.V.Names[v2]].String())
		in.Save("_temp_a", converted)
		return v1, "_temp_a"
	} else if t1 == LIST && t2 == ARR {
		// in.v.Lists["_temp_a"] = Listify(in.v.Arrs[v2])
		// in.v.Types["_temp_a"] = LIST
		return v1, "_temp_a"
	} else if t1 == ARR && t2 == LIST {
		// in.v.Lists["_temp_a"] = Listify(in.v.Arrs[v1])
		// in.v.Types["_temp_a"] = LIST
		return "_temp_a", v2
	} else if t1 == BYTE && t2 == INT {
		converted := big.NewInt(int64(in.V.Bytes[in.V.Names[v1]]))
		in.Save("_temp_a", converted)
		return "_temp_a", v2
	} else if t1 == INT && t2 == BYTE {
		converted := big.NewInt(int64(in.V.Bytes[in.V.Names[v2]]))
		in.Save("_temp_a", converted)
		return v1, "_temp_a"
	} else if t1 == BYTE && t2 == FLOAT {
		converted := big.NewFloat(float64(in.V.Bytes[in.V.Names[v1]]))
		in.Save("_temp_a", converted)
		return "_temp_a", v2
	} else if t1 == FLOAT && t2 == BYTE {
		converted := big.NewFloat(float64(in.V.Bytes[in.V.Names[v2]]))
		in.Save("_temp_a", converted)
		return v1, "_temp_a"
	}
	return v1, v2
}

func (in *Interpreter) Error(act bytecode.Action, message, etype string) {
	// zero_division
	// index
	// undeclared
	// arg_count
	// arg_type
	// sys
	// file
	// permission
	if !in.IgnoreErr {
		fmt.Printf("Runtime error: %s\nLocation: line %d\nAction: %s\nType: %s\nLine:\n%s\n", message, act.Source.N+1, act.Type, etype, strings.ReplaceAll(act.Source.Source, "\r\n", "\n"))
	}
	in.ErrSource = act.Source
	error_type = etype
	error_message = message
}

func (in *Interpreter) CheckArgN(action bytecode.Action, minimal, maximal int) bool {
	switch {
	case minimal == maximal:
		if len(action.Variables) != minimal {
			in.Error(action, fmt.Sprintf("%d arguments were provided, expected %d!", len(action.Variables), minimal), "arg_count")
			return true
		}
	case maximal == -1:
		if len(action.Variables) < minimal {
			in.Error(action, fmt.Sprintf("%d arguments were provided, expected not less than %d!", len(action.Variables), minimal), "arg_count")
			return true
		}
	default:
		if len(action.Variables) < minimal || len(action.Variables) > maximal {
			in.Error(action, fmt.Sprintf("%d arguments were provided, expected between %d and %d!", len(action.Variables), minimal, maximal), "arg_count")
			return true
		}
	}
	return false
}

func (in *Interpreter) CheckDtype(action bytecode.Action, index int, dtypes ...byte) bool {
	dstrings := map[byte]string{0: "noth", 1: "int", 2: "float", 3: "str", 4: "arr", 5: "list", 6: "pair", 7: "bool", 8: "byte", 9: "func", 10: "id"}
	found := false
	for _, dtype := range dtypes {
		if in.V.Types[in.V.Names[string(action.Variables[index])]] == dtype {
			found = true
			break
		}
	}
	if !found {
		l := bytecode.List{}
		for _, dtype := range dtypes {
			ListAppend(&l, in, dstrings[dtype])
		}
		in.Error(action, fmt.Sprintf("argument %d (%s) is %s, must be one of: %s!", index, string(action.Variables[index]), dstrings[in.V.Types[in.V.Names[string(action.Variables[index])]]], ListString(&l, in)), "arg_type")
		return true
	}
	return false
}

func (in *Interpreter) GetAny(var_name string) any {
	switch in.V.Types[in.V.Names[var_name]] {
	case INT:
		return in.V.Ints[in.V.Names[var_name]]
	case FLOAT:
		return in.V.Floats[in.V.Names[var_name]]
	case STR:
		return in.V.Strs[in.V.Names[var_name]]
	case LIST:
		return in.V.Lists[in.V.Names[var_name]]
	case BOOL:
		return in.V.Bools[in.V.Names[var_name]]
	case BYTE:
		return in.V.Bytes[in.V.Names[var_name]]
	case ARR:
		return in.V.Arrs[in.V.Names[var_name]]
	case FUNC:
		return in.V.Funcs[in.V.Names[var_name]]
	case PAIR:
		return in.V.Pairs[in.V.Names[var_name]]
	}
	return nil
}

func (in *Interpreter) GetAnyRef(ref uint64) any { // TODO: add all types
	switch in.V.Types[ref] {
	case INT:
		return in.V.Ints[ref]
	case FLOAT:
		return in.V.Floats[ref]
	case STR:
		return in.V.Strs[ref]
	case LIST:
		return in.V.Lists[ref]
	case BOOL:
		return in.V.Bools[ref]
	case BYTE:
		return in.V.Bytes[ref]
	case ARR:
		return in.V.Arrs[ref]
	case FUNC:
		return in.V.Funcs[ref]
	case PAIR:
		return in.V.Pairs[ref]
	}
	return nil
}

func (in *Interpreter) RemoveName(name string) {
	id, ok := in.V.Names[name]
	if ok {
		in.RemoveId(id)
		delete(in.V.Names, name)
		delete(in.V.Types, id)
	}
}

func (in *Interpreter) RemoveId(id uint64) {
	switch in.V.Types[id] {
	case INT:
		delete(in.V.Ints, id)
	case FLOAT:
		delete(in.V.Floats, id)
	case STR:
		delete(in.V.Strs, id)
	case BYTE:
		delete(in.V.Bytes, id)
	case BOOL:
		delete(in.V.Bools, id)
	case FUNC:
		delete(in.V.Funcs, id)
	case ARR:
		delete(in.V.Arrs, id)
	case LIST:
		// TODO: verify if needed
		/*
			for _, idlet := range in.V.Lists[id].Ids {
				in.RemoveId(idlet)
			}
		*/
		delete(in.V.Lists, id)
	case PAIR:
		/*
			for key, idlet := range in.V.Pairs[id].Ids {
				in.RemoveId(idlet)
				delete(in.V.Pairs[id].Ids, key)
			}
		*/
		delete(in.V.Pairs, id)
	case SPAN:
		delete(in.V.Spans, id)
	}
}

func (in *Interpreter) GC() {
	used := []uint64{}
	for name, val := range in.V.Names {
		if strings.HasPrefix(name, "_temp_") {
			continue
		}
		used = append(used, val)
		if in.V.Types[val] == LIST {
			used = append(used, in.V.Lists[val].Ids...)
		} else if in.V.Types[val] == ARR {
			used = append(used, in.V.Arrs[val].Ids...)
		} else if in.V.Types[val] == PAIR {
			var idlet []uint64
			for _, v := range in.V.Pairs[val].Ids {
				idlet = append(idlet, v)
			}
			used = append(used, idlet...)
		}
	}
	for key, dtype := range in.V.Types {
		if !bytecode.Has(used, key) {
			for name, uval := range in.V.Names {
				// break // diable name cleanup
				if uval == key {
					delete(in.V.Names, name)
				}
			}
			switch dtype {
			case INT:
				delete(in.V.Ints, key)
			case FLOAT:
				delete(in.V.Floats, key)
			case STR:
				delete(in.V.Strs, key)
			case LIST:
				delete(in.V.Lists, key)
			case PAIR:
				delete(in.V.Pairs, key)
			case BOOL:
				delete(in.V.Bools, key)
			case BYTE:
				delete(in.V.Bytes, key)
			case ID:
				delete(in.V.Ids, key)
			case FUNC:
				delete(in.V.Funcs, key)
			}
			// remove var type
			delete(in.V.Types, key)
		}
	}
}

func (in *Interpreter) GC2() {
	usedSet := make(map[uint64]struct{}, len(in.V.Names))

	// Recursively mark all reachable IDs
	var markUsed func(uint64)
	markUsed = func(id uint64) {
		if _, exists := usedSet[id]; exists {
			return
		}
		usedSet[id] = struct{}{}

		switch in.V.Types[id] {
		case LIST:
			if list, exists := in.V.Lists[id]; exists {
				for _, childID := range list.Ids {
					markUsed(childID)
				}
			}
		case SPAN:
			if span, exists := in.V.Spans[id]; exists {
				for childID := span.Start; childID < span.Start+span.Length; childID++ {
					markUsed(childID)
				}
			}
		case PAIR:
			if pair, exists := in.V.Pairs[id]; exists {
				for _, childID := range pair.Ids {
					markUsed(childID)
				}
			}
		}
	}

	// Step 1: Initial marking from root variable names
	for _, id := range in.V.Names {
		markUsed(id)
	}

	// Step 2: Clean up names
	for name, id := range in.V.Names {
		if _, exists := usedSet[id]; !exists {
			delete(in.V.Names, name)
		}
	}

	// Step 3: Clean up unused objects
	for id := range in.V.Types {
		if _, exists := usedSet[id]; exists {
			continue
		}

		switch in.V.Types[id] {
		case INT:
			delete(in.V.Ints, id)
		case FLOAT:
			delete(in.V.Floats, id)
		case STR:
			delete(in.V.Strs, id)
		case LIST:
			delete(in.V.Lists, id)
		case ARR:
			delete(in.V.Arrs, id)
		case SPAN:
			delete(in.V.Spans, id)
		case PAIR:
			delete(in.V.Pairs, id)
		case BOOL:
			delete(in.V.Bools, id)
		case BYTE:
			delete(in.V.Bytes, id)
		case ID:
			delete(in.V.Ids, id)
		case FUNC:
			delete(in.V.Funcs, id)
		}
		delete(in.V.Types, id)
	}
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

func (in *Interpreter) Parse2(str string) []string {
	reg_var := regexp.MustCompile(`\{.+?\}`)
	for _, match := range reg_var.FindAllString(str, -1) {
		code := match[1 : len(match)-1]
		if variable, ok := in.V.Names[code]; ok {
			a := in.GetAnyRef(variable)
			text := ""
			switch v := a.(type) {
			case string:
				text = v
			case *big.Int:
				text = v.String()
			case *big.Float:
				text = v.String()
			case byte:
				text = fmt.Sprintf("%d", v)
			case bool:
				text = ternary(v, "true", "false")
			case bytecode.Function:
				text = "func." + v.Name
			case bytecode.List:
				text = ListString(&v, in)
			case bytecode.Array:
				text = v.String()
			case bytecode.Pair:
				text = PairString(&v, in)
			}
			str = strings.ReplaceAll(str, match, text)
		} else {
			// TODO: ###
			in_p := NewInterpreter(code, ".")
			for name := range in.V.Names {
				in_p.Save(name, in.GetAny(name))
			}
			node_name := fmt.Sprintf("_node_%d", bytecode.NodeN-1)
			in_p.Code[node_name] = in_p.Code[node_name][:len(in_p.Code[node_name])-1]
			in_p.Run(node_name)
			a := in_p.GetAny(in_p.Code[node_name][len(in_p.Code[node_name])-1].Target)
			text := ""
			switch v := a.(type) {
			case string:
				text = v
			case *big.Int:
				text = v.String()
			case *big.Float:
				text = v.String()
			case byte:
				text = fmt.Sprintf("%d", v)
			case bool:
				text = ternary(v, "true", "false")
			case bytecode.Function:
				text = "func." + v.Name
			case bytecode.List:
				text = ListString(&v, in)
			case bytecode.Array:
				text = v.String()
			case bytecode.Pair:
				text = PairString(&v, in)
			}
			str = strings.ReplaceAll(str, match, text)
			in_p.Destroy()
		}
	}
	reg_parts := regexp.MustCompile(`(".*?"|[^\s]+)`)
	return reg_parts.FindAllString(str, -1)
}

func (in *Interpreter) Fmt(str string) string {
	reg_var := regexp.MustCompile(`\{.+?\}`)
	for _, match := range reg_var.FindAllString(str, -1) {
		code := match[1 : len(match)-1]
		if variable, ok := in.V.Names[code]; ok {
			a := in.GetAnyRef(variable)
			text := ""
			switch v := a.(type) {
			case string:
				text = v
			case *big.Int:
				text = v.String()
			case *big.Float:
				text = v.String()
			case byte:
				text = fmt.Sprintf("%d", v)
			case bool:
				text = ternary(v, "true", "false")
			case bytecode.Function:
				text = "func." + v.Name
			case bytecode.List:
				text = ListString(&v, in)
			case bytecode.Array:
				text = v.String()
			case bytecode.Pair:
				text = PairString(&v, in)
			}
			str = strings.ReplaceAll(str, match, text)
		} else {
			// TODO: ###
			in_p := NewInterpreter(code, ".")
			for name := range in.V.Names {
				in_p.Save(name, in.GetAny(name))
			}
			node_name := fmt.Sprintf("_node_%d", bytecode.NodeN-1)
			in_p.Code[node_name] = in_p.Code[node_name][:len(in_p.Code[node_name])-1]
			in_p.Run(node_name)
			a := in_p.GetAny(in_p.Code[node_name][len(in_p.Code[node_name])-1].Target)
			text := ""
			switch v := a.(type) {
			case string:
				text = v
			case *big.Int:
				text = v.String()
			case *big.Float:
				text = v.String()
			case byte:
				text = fmt.Sprintf("%d", v)
			case bool:
				text = ternary(v, "true", "false")
			case bytecode.Function:
				text = "func." + v.Name
			case bytecode.List:
				text = ListString(&v, in)
			case bytecode.Array:
				text = v.String()
			case bytecode.Pair:
				text = PairString(&v, in)
			}
			str = strings.ReplaceAll(str, match, text)
		}
	}
	return str
}

func (in *Interpreter) StrToList(str_name, list_name string) {
	str := in.GetAny(str_name).(string)
	l := bytecode.List{}
	for _, item := range str {
		l.Ids = append(l.Ids, in.GetRef(string(item)))
	}
	in.Save(list_name, l)
	//in.CopyList(list_name, in)
}

func (in *Interpreter) Parse(str string) []string {
	reg_var := regexp.MustCompile(`\{.+?\}`)
	parsed, m := remove_strings(str)
	splitted := strings.Split(parsed, " ")
	var sanitized, processed []string
	for _, splitlet := range splitted {
		if splitlet != "" {
			if !(strings.HasPrefix(splitlet, "\"") && strings.HasSuffix(splitlet, "\"")) {
				sanitized = append(sanitized, splitlet)
				continue
			}
			part, ok := m[splitlet[1:len(splitlet)-1]]
			if ok {
				sanitized = append(sanitized, part)
			} else {
				sanitized = append(sanitized, splitlet)
			}
		}
	}
	for _, sanlet := range sanitized {
		if strings.HasPrefix(sanlet, "{") && strings.HasSuffix(sanlet, "}") {
			code := sanlet[1 : len(sanlet)-1]
			var targ_str string
			if dtype, ok := in.V.Types[in.V.Names[code]]; ok {
				switch dtype {
				case INT:
					targ_str = in.V.Ints[in.V.Names[code]].String()
				case FLOAT:
					targ_str = in.V.Floats[in.V.Names[code]].String()
				case STR:
					targ_str = in.V.Strs[in.V.Names[code]]
				case BYTE:
					targ_str = fmt.Sprintf("%d", in.V.Bytes[in.V.Names[code]])
				}
				processed = append(processed, targ_str)
				continue
			}
			/*
				in_str := NewInterpreter(code, "string: "+str)
				in_str.V = in.V
			*/
			in_str := Interpreter{}
			in_str.Copy(in)
			/*
				for n, fn_arg := range fn.Vars {
					fn_arg_str := string(fn_arg)
					if in.V.Types[in.V.Names[string(action.Variables[n])]] == PAIR {
						p := f_in.CopyPair(string(action.Variables[n]), in)
						f_in.Save(fn_arg_str, p)
					} else if in.V.Types[in.V.Names[string(action.Variables[n])]] == LIST {
						l := f_in.CopyList(string(action.Variables[n]), in)
						f_in.Save(fn_arg_str, l)
					} else {
						f_in.Save(fn_arg_str, in.GetAny(string(action.Variables[n])))
					}
				}
			*/
			in_str.Run(fmt.Sprintf("_node_%d", bytecode.NodeN-1))
			node := in_str.Code[fmt.Sprintf("_node_%d", bytecode.NodeN-1)]
			targ := node[len(node)-1].Target
			// TODO: add more types
			switch in_str.V.Types[in.V.Names[targ]] {
			case INT:
				targ_str = in_str.V.Ints[in.V.Names[targ]].String()
			case FLOAT:
				targ_str = in_str.V.Floats[in.V.Names[targ]].String()
			case STR:
				targ_str = in_str.V.Strs[in.V.Names[targ]]
			case BYTE:
				targ_str = fmt.Sprintf("%d", in_str.V.Bytes[in.V.Names[targ]])
			}
			processed = append(processed, targ_str)
		} else {
			processed = append(processed, sanlet)
		}
	}
	for n, proclet := range processed {
		for _, m := range reg_var.FindAllString(proclet, -1) {
			code := m[1 : len(m)-1]
			var targ_str string
			if dtype, ok := in.V.Types[in.V.Names[code]]; ok {
				switch dtype {
				case INT:
					targ_str = in.V.Ints[in.V.Names[code]].String()
				case FLOAT:
					targ_str = in.V.Floats[in.V.Names[code]].String()
				case STR:
					targ_str = in.V.Strs[in.V.Names[code]]
				case BYTE:
					targ_str = fmt.Sprintf("%d", in.V.Bytes[in.V.Names[code]])
				}
				processed = append(processed, targ_str)
				continue
			}
			in_str := NewInterpreter(code, "string: "+str)
			in_str.V = in.V
			in_str.Run(fmt.Sprintf("_node_%d", bytecode.NodeN-1))
			node := in_str.Code[fmt.Sprintf("_node_%d", bytecode.NodeN-1)]
			targ := node[len(node)-1].Target
			// TODO: add more types
			switch in_str.V.Types[in.V.Names[targ]] {
			case INT:
				targ_str = in_str.V.Ints[in.V.Names[targ]].String()
			case FLOAT:
				targ_str = in_str.V.Floats[in.V.Names[targ]].String()
			case STR:
				targ_str = in_str.V.Strs[in.V.Names[targ]]
			case BYTE:
				targ_str = fmt.Sprintf("%d", in_str.V.Bytes[in.V.Names[targ]])
			}
			processed[n] = strings.ReplaceAll(processed[n], m, targ_str)
		}
	}
	fmt.Println(processed) // ### TODO: delet
	return processed
}

func (in *Interpreter) ParseStr(str string) string {
	processed := str
	r := regexp.MustCompile(`\{[^\\]+?\}`)
	patterns := r.FindAllString(processed, -1)
	for _, sanlet := range patterns {
		code := sanlet[1 : len(sanlet)-1]
		var targ_str string
		if dtype, ok := in.V.Types[in.V.Names[code]]; ok {
			switch dtype {
			case INT:
				targ_str = in.V.Ints[in.V.Names[code]].String()
			case FLOAT:
				targ_str = in.V.Floats[in.V.Names[code]].String()
			case STR:
				targ_str = in.V.Strs[in.V.Names[code]]
			case BYTE:
				targ_str = fmt.Sprintf("%d", in.V.Bytes[in.V.Names[code]])
			}
			processed = strings.ReplaceAll(processed, sanlet, targ_str)
			continue
		}
		println(code)
		in_str := NewInterpreter(code, "string: "+str)
		in_str.V = in.V
		in_str.Run(fmt.Sprintf("_node_%d", bytecode.NodeN-1))
		node := in_str.Code[fmt.Sprintf("_node_%d", bytecode.NodeN-1)]
		targ := node[len(node)-1].Target
		// TODO: add more types
		switch in_str.V.Types[in.V.Names[targ]] {
		case INT:
			targ_str = in_str.V.Ints[in.V.Names[targ]].String()
		case FLOAT:
			targ_str = in_str.V.Floats[in.V.Names[targ]].String()
		case STR:
			targ_str = in_str.V.Strs[in.V.Names[targ]]
		case BYTE:
			targ_str = fmt.Sprintf("%d", in_str.V.Bytes[in.V.Names[targ]])
		}
		processed = strings.ReplaceAll(processed, sanlet, targ_str)
	}
	return processed
}

func (in *Interpreter) CopyList(vname string, og *Interpreter) bytecode.List {
	l := og.GetAny(vname).(bytecode.List)
	lnew := bytecode.List{}
	for _, id := range l.Ids {
		a := og.GetAnyRef(id)
		newid := uint64(0)
		ok := true
		for ok {
			newid = Uint64()
			_, ok = og.V.Types[newid]
		}
		in.SaveRef(newid, a)
		lnew.Ids = append(lnew.Ids, newid)
	}
	return lnew
}

func (in *Interpreter) CopyPair(vname string, og *Interpreter) bytecode.Pair {
	l := og.GetAny(vname).(bytecode.Pair)
	lnew := bytecode.Pair{}
	lnew.Ids = make(map[string]uint64)
	for key, id := range l.Ids {
		a := og.GetAnyRef(id)
		newid := uint64(0)
		ok := true
		for ok {
			newid = Uint64()
			_, ok = og.V.Types[newid]
		}
		in.SaveRef(newid, a)
		//lnew.Ids = append(lnew.Ids, newid)
		lnew.Ids[key] = newid
	}
	return lnew
}

func (in *Interpreter) Run(node_name string) bool {
	actions := in.Code[node_name]
	focus := 0
	for focus < len(actions) && !in.halt {
		action := actions[focus]
		switch action.Type {
		case "const":
			reg_int := regexp.MustCompile(`^-?[0-9]+$`)
			reg_float := regexp.MustCompile(`^-?[0-9]+\.[0-9]+$`)
			reg_byte := regexp.MustCompile(`^-?b\.[0-9]+$`)
			switch {
			case reg_float.MatchString(string(actions[focus].Variables[0])):
				//f64, _ := strconv.ParseFloat(string(actions[focus].Variables[0]), 64)
				b := big.NewFloat(0)
				b.SetString(string(actions[focus].Variables[0]))
				in.Save(actions[focus].Target, b)
			case reg_byte.MatchString(string(actions[focus].Variables[0])):
				i64, _ := strconv.ParseInt(string(actions[focus].Variables[0])[2:], 10, 64)
				in.Save(actions[focus].Target, byte(i64))
			case reg_int.MatchString(string(actions[focus].Variables[0])):
				b := big.NewInt(0)
				b.SetString(string(actions[focus].Variables[0]), 10)
				in.Save(actions[focus].Target, b)
			case string(actions[focus].Variables[0]) == "true" || string(actions[focus].Variables[0]) == "false":
				in.Save(action.Target, string(actions[focus].Variables[0]) == "true")
			default:
				in.Save(actions[focus].Target, string(actions[focus].Variables[0])[1:len(string(actions[focus].Variables[0]))-1])
			}
		case "return":
			if len(action.Variables) == 1 {
				in.Save("_return_", in.GetAny(string(action.Variables[0])))
			} else if len(action.Variables) > 1 {
				l := bytecode.List{}
				for n := range len(action.Variables) {
					ListAppend(&l, in, in.GetAny(string(action.Variables[n])))
				}
				in.Save("_return_", l)
			}
			in.halt = true
		case "func":
			name := string(actions[focus].Variables[0])
			fn := bytecode.Function{name, actions[focus].Target, actions[focus].Variables[1:], actions[focus].Target}
			in.Save(name, fn)
		case "++":
			switch in.V.Types[in.V.Names[string(action.Variables[0])]] {
			case INT:
				i := big.NewInt(0)
				i.Set(in.V.Ints[in.V.Names[string(action.Variables[0])]])
				in.Save(actions[focus].Target, i)
				in.V.Ints[in.V.Names[actions[focus].Target]].Add(in.V.Ints[in.V.Names[string(action.Variables[0])]], big.NewInt(1))
			case FLOAT:
				in.Save(actions[focus].Target, in.V.Floats[in.V.Names[string(action.Variables[0])]])
				in.V.Floats[in.V.Names[actions[focus].Target]].Add(in.V.Floats[in.V.Names[string(action.Variables[0])]], big.NewFloat(1))
			case BYTE:
				result := in.V.Bytes[in.V.Names[string(action.Variables[0])]] + 1
				in.Save(actions[focus].Target, result)
			}
		case "--":
			switch in.V.Types[in.V.Names[string(action.Variables[0])]] {
			case INT:
				in.Save(actions[focus].Target, in.V.Ints[in.V.Names[string(action.Variables[0])]])
				in.V.Ints[in.V.Names[actions[focus].Target]].Sub(in.V.Ints[in.V.Names[string(action.Variables[0])]], big.NewInt(1))
			case FLOAT:
				in.Save(actions[focus].Target, in.V.Floats[in.V.Names[string(action.Variables[0])]])
				in.V.Floats[in.V.Names[actions[focus].Target]].Sub(in.V.Floats[in.V.Names[string(action.Variables[0])]], big.NewFloat(1))
			case BYTE:
				result := in.V.Bytes[in.V.Names[string(action.Variables[0])]] - 1
				in.Save(actions[focus].Target, result)
			}
		case "for":
			sources := []bytecode.List{}
			targets := []string{}
			for n := 0; n < len(action.Variables); n += 2 {
				fn := fmt.Sprintf("_for%s%d", action.Target, n/2)
				if in.V.Types[in.V.Names[string(action.Variables[n])]] == STR {
					in.StrToList(string(actions[focus].Variables[n]), fn)
				} else if in.V.Types[in.V.Names[string(action.Variables[n])]] == LIST {
					l := in.CopyList(string(action.Variables[n]), in)
					in.Save(fn, l)
				}
				// in.Save(fn, in.V.Lists[in.V.Names[string(action.Variables[n])]])
				sources = append(sources, in.V.Lists[in.V.Names[fn]])
				targets = append(targets, string(action.Variables[n+1]))
			}
			for ind := 0; ind < len(sources[0].Ids); ind++ {
				for n := 0; n < len(sources); n++ {
					in.Save(targets[n], in.GetAnyRef(sources[n].Ids[ind]))
				}
				err := in.Run(action.Target)
				if err {
					return true
				}
			}
			for n := 0; n < len(action.Variables); n += 2 {
				fn := fmt.Sprintf("_for%s%d", action.Target, n/2)
				in.RemoveName(fn)
			}
		case "+":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Types[in.V.Names[o]] {
			case INT:
				in.Save(actions[focus].Target, big.NewInt(0))
				in.V.Ints[in.V.Names[actions[focus].Target]].Add(in.V.Ints[in.V.Names[o]], in.V.Ints[in.V.Names[t]])
			case FLOAT:
				in.Save(actions[focus].Target, big.NewFloat(0))
				in.V.Floats[in.V.Names[actions[focus].Target]].Add(in.V.Floats[in.V.Names[o]], in.V.Floats[in.V.Names[t]])
			case STR:
				err := in.CheckDtype(action, 1, STR)
				if err {
					return true
				}
				result := in.V.Strs[in.V.Names[o]] + in.V.Strs[in.V.Names[t]]
				in.Save(actions[focus].Target, result)
			case BYTE:
				result := in.V.Bytes[in.V.Names[o]] + in.V.Bytes[in.V.Names[t]]
				in.Save(actions[focus].Target, result)
			}
		case "-":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Types[in.V.Names[o]] {
			case INT:
				in.Save(actions[focus].Target, big.NewInt(0))
				in.V.Ints[in.V.Names[actions[focus].Target]].Sub(in.V.Ints[in.V.Names[o]], in.V.Ints[in.V.Names[t]])
			case FLOAT:
				in.Save(actions[focus].Target, big.NewFloat(0))
				in.V.Floats[in.V.Names[actions[focus].Target]].Sub(in.V.Floats[in.V.Names[o]], in.V.Floats[in.V.Names[t]])
			case BYTE:
				result := in.V.Bytes[in.V.Names[o]] - in.V.Bytes[in.V.Names[t]]
				in.Save(actions[focus].Target, result)
			}
		case "^":
			one, two := in.EqualizeTypes(string(action.Variables[0]), string(action.Variables[1]))
			switch in.V.Types[in.V.Names[one]] {
			case INT:
				in.Save(action.Target, in.V.Ints[in.V.Names[one]])
				in.V.Ints[in.V.Names[action.Target]] = PowInt(in.V.Ints[in.V.Names[action.Target]], in.V.Ints[in.V.Names[two]])
			case FLOAT:
				o, _ := in.V.Floats[in.V.Names[one]].Float64()
				t, _ := in.V.Floats[in.V.Names[two]].Float64()
				in.Save(action.Target, big.NewFloat(math.Pow(o, t)))
			case BYTE:
				in.V.Bytes[in.V.Names[action.Target]] = byte(math.Pow(float64(in.V.Bytes[in.V.Names[string(action.Variables[0])]]), float64(in.V.Bytes[in.V.Names[string(action.Variables[1])]])))
				in.V.Types[in.V.Names[action.Target]] = BYTE
			}
		case "==":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Types[in.V.Names[o]] {
			case INT:
				r := in.V.Ints[in.V.Names[o]].Cmp(in.V.Ints[in.V.Names[t]])
				in.Save(actions[focus].Target, r == 0)
			case FLOAT:
				r := in.V.Floats[in.V.Names[o]].Cmp(in.V.Floats[in.V.Names[t]])
				in.Save(actions[focus].Target, r == 0)
			case BYTE:
				in.Save(action.Target, in.V.Bytes[in.V.Names[string(action.Variables[0])]] == in.V.Bytes[in.V.Names[string(action.Variables[1])]])
			case STR:
				in.Save(action.Target, in.V.Strs[in.V.Names[o]] == in.V.Strs[in.V.Names[t]])
			}
		case "!=":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Types[in.V.Names[o]] {
			case INT:
				r := in.V.Ints[in.V.Names[o]].Cmp(in.V.Ints[in.V.Names[t]])
				in.Save(actions[focus].Target, r != 0)
			case FLOAT:
				r := in.V.Floats[in.V.Names[o]].Cmp(in.V.Floats[in.V.Names[t]])
				in.Save(actions[focus].Target, r != 0)
			case BYTE:
				in.Save(action.Target, in.V.Bytes[in.V.Names[string(action.Variables[0])]] != in.V.Bytes[in.V.Names[string(action.Variables[1])]])
			case STR:
				in.Save(action.Target, in.V.Strs[in.V.Names[string(action.Variables[0])]] != in.V.Strs[in.V.Names[string(action.Variables[1])]])
			}
		case "<":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Types[in.V.Names[o]] {
			case INT:
				r := in.V.Ints[in.V.Names[o]].Cmp(in.V.Ints[in.V.Names[t]])
				in.Save(actions[focus].Target, r == -1)
			case FLOAT:
				r := in.V.Floats[in.V.Names[o]].Cmp(in.V.Floats[in.V.Names[t]])
				in.Save(actions[focus].Target, r == -1)
			case BYTE:
				in.Save(action.Target, in.V.Bytes[in.V.Names[string(action.Variables[0])]] < in.V.Bytes[in.V.Names[string(action.Variables[1])]])
			}
		case ">":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Types[in.V.Names[o]] {
			case INT:
				r := in.V.Ints[in.V.Names[o]].Cmp(in.V.Ints[in.V.Names[t]])
				in.Save(actions[focus].Target, r == 1)
			case FLOAT:
				r := in.V.Floats[in.V.Names[o]].Cmp(in.V.Floats[in.V.Names[t]])
				in.Save(actions[focus].Target, r == 1)
			case BYTE:
				in.Save(action.Target, in.V.Bytes[in.V.Names[string(action.Variables[0])]] > in.V.Bytes[in.V.Names[string(action.Variables[1])]])
			}
		case ".":
			if in.V.Types[in.V.Names[string(actions[focus].Variables[0])]] == INT && in.V.Types[in.V.Names[string(actions[focus].Variables[1])]] == INT {
				in.Save(actions[focus].Target, big.NewFloat(0))
				in.V.Floats[in.V.Names[actions[focus].Target]].SetString(fmt.Sprintf("%s.%s", in.V.Ints[in.V.Names[string(actions[focus].Variables[0])]].String(), in.V.Ints[in.V.Names[string(actions[focus].Variables[1])]].String()))
			} else {
				switch in.V.Types[in.V.Names[string(actions[focus].Variables[0])]] {
				case PAIR:
					p := in.V.Pairs[in.V.Names[string(actions[focus].Variables[0])]]
					ind := PairKey(in, string(actions[focus].Variables[1]))
					in.V.Names[actions[focus].Target] = p.Ids[ind]
				}
			}
		case "'", "''":
			// TODO: add index errors
			err := in.CheckDtype(actions[focus], 0, LIST, ARR, PAIR, STR)
			if err {
				return true
			}
			err = in.CheckDtype(actions[focus], 1, INT, STR, LIST)
			if err {
				return true
			}
			switch in.V.Types[in.V.Names[string(actions[focus].Variables[0])]] {
			case ARR:
				// TODO: complete all types
				a := in.V.Arrs[in.V.Names[string(actions[focus].Variables[0])]]
				switch a.Dtype {
				case INT:
					ind := in.V.Ints[in.V.Names[string(actions[focus].Variables[1])]].Int64()
					if ind < 0 {
						ind += int64(len(a.Ints))
					}
					in.Save(action.Target, a.Ints[ind])
				case STR:
					ind := in.V.Ints[in.V.Names[string(actions[focus].Variables[1])]].Int64()
					if ind < 0 {
						ind += int64(len(a.Strs))
					}
					in.Save(action.Target, a.Strs[ind])
				case BYTE:
					ind := in.V.Ints[in.V.Names[string(actions[focus].Variables[1])]].Int64()
					if ind < 0 {
						ind += int64(len(a.Bytes))
					}
					in.Save(action.Target, a.Bytes[ind])
				}
			case STR:
				if in.V.Types[in.V.Names[string(actions[focus].Variables[1])]] == INT {
					s := in.V.Strs[in.V.Names[string(action.Variables[0])]]
					ind := int(in.V.Ints[in.V.Names[string(actions[focus].Variables[1])]].Int64())
					length := 0
					for range s {
						length++
					}
					if ind < 0 {
						ind += length
					}
					// in.V.Names[actions[focus].Target] = l.Ids[ind]
					saved := false
					for i, r := range s {
						if i == ind {
							in.Save(action.Target, string(r))
							saved = true
							break
						}
					}
					if !saved {
						in.Error(action, "string index error!", "index")
						return true
					}
				} else if in.V.Types[in.V.Names[string(actions[focus].Variables[1])]] == LIST {
					s := in.V.Strs[in.V.Names[string(action.Variables[0])]]
					ns := ""
					length := 0
					runes := []string{}
					for _, rune := range s {
						runes = append(runes, string(rune))
						length++
					}
					inds := in.V.Lists[in.V.Names[string(actions[focus].Variables[1])]]
					for _, id := range inds.Ids {
						if in.V.Types[id] != INT {
							in.Error(action, "Impossible index within list!", "index")
							return true
						}
						i := in.V.Ints[id].Int64()
						if i < 0 {
							i += int64(length)
						}
						if i < 0 || int(i) >= len(runes) {
							in.Error(action, "index out of range!", "index")
							return true
						}
						ns += runes[i]
					}
					in.Save(action.Target, ns)
				}
			case LIST:
				if in.V.Types[in.V.Names[string(actions[focus].Variables[1])]] == INT {
					l := in.V.Lists[in.V.Names[string(action.Variables[0])]]
					ind := in.V.Ints[in.V.Names[string(actions[focus].Variables[1])]].Int64()
					if ind < 0 {
						ind += int64(len(l.Ids))
					}
					if action.Type == "'" {
						in.Save(action.Target, in.GetAnyRef(l.Ids[ind]))
					} else {
						// if ''
						in.V.Names[actions[focus].Target] = l.Ids[ind]
					}
					//l.Ids[ind] = in.V.Names[action.Target]
					//in.V.Lists[in.V.Names[string(action.Variables[0])]] = l
					// TODO: evaluate approach below or above
					//in.V.Names[actions[focus].Target] = l.Ids[ind]
				} else if in.V.Types[in.V.Names[string(actions[focus].Variables[1])]] == LIST {
					l := in.V.Lists[in.V.Names[string(action.Variables[0])]]
					inds := in.V.Lists[in.V.Names[string(actions[focus].Variables[1])]]
					nl := bytecode.List{}
					for _, id := range inds.Ids {
						if in.V.Types[id] != INT {
							in.Error(action, "Impossible index within list!", "index")
							return true
						}
						i := in.V.Ints[id].Int64()
						if i < 0 {
							i += int64(len(l.Ids))
						}
						ListAppend(&nl, in, in.GetAnyRef(l.Ids[i]))
					}
					in.Save(action.Target, nl)
				}
			case PAIR:
				p := in.V.Pairs[in.V.Names[string(actions[focus].Variables[0])]]
				ind := PairKey(in, in.GetAny(string(actions[focus].Variables[1])))
				if action.Type == "'" {
					in.Save(action.Target, in.GetAnyRef(p.Ids[ind]))
				} else {
					in.V.Names[actions[focus].Target] = p.Ids[ind]
				}
			}
		case "*":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Types[in.V.Names[o]] {
			case INT:
				in.Save(actions[focus].Target, big.NewInt(0))
				in.V.Ints[in.V.Names[actions[focus].Target]].Mul(in.V.Ints[in.V.Names[o]], in.V.Ints[in.V.Names[t]])
			case FLOAT:
				in.Save(actions[focus].Target, big.NewFloat(0))
				in.V.Floats[in.V.Names[actions[focus].Target]].Mul(in.V.Floats[in.V.Names[o]], in.V.Floats[in.V.Names[t]])
			case BYTE:
				result := in.V.Bytes[in.V.Names[o]] * in.V.Bytes[in.V.Names[t]]
				in.Save(actions[focus].Target, result)
			}
		case "/":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Types[in.V.Names[o]] {
			case INT:
				in.Save(actions[focus].Target, big.NewInt(0))
				in.V.Ints[in.V.Names[actions[focus].Target]].Quo(in.V.Ints[in.V.Names[o]], in.V.Ints[in.V.Names[t]])
			case FLOAT:
				in.Save(actions[focus].Target, big.NewFloat(0))
				in.V.Floats[in.V.Names[actions[focus].Target]].Quo(in.V.Floats[in.V.Names[o]], in.V.Floats[in.V.Names[t]])
			case BYTE:
				result := in.V.Bytes[in.V.Names[o]] / in.V.Bytes[in.V.Names[t]]
				in.Save(actions[focus].Target, result)
			}
		case "%":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Types[in.V.Names[o]] {
			case INT:
				in.Save(actions[focus].Target, big.NewInt(0))
				in.V.Ints[in.V.Names[actions[focus].Target]].Mod(in.V.Ints[in.V.Names[o]], in.V.Ints[in.V.Names[t]])
			case FLOAT:
				//TODO: revisit later
				in.Error(action, "unsupported operation as of now", "TODO")
				return true
				// in.Save(actions[focus].Target, big.NewFloat(0))
				// in.V.Floats[in.V.Names[actions[focus].Target]].Mod(in.V.Floats[in.V.Names[o]], in.V.Floats[in.V.Names[t]])
			case BYTE:
				result := in.V.Bytes[in.V.Names[o]] / in.V.Bytes[in.V.Names[t]]
				in.Save(actions[focus].Target, result)
			}
		case "//":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Types[in.V.Names[o]] {
			case INT:
				in.Save(actions[focus].Target, big.NewInt(0))
				in.V.Ints[in.V.Names[actions[focus].Target]].QuoRem(in.V.Ints[in.V.Names[o]], in.V.Ints[in.V.Names[t]], in.V.Ints[in.V.Names[actions[focus].Target]])
			case FLOAT:
				//TODO: revisit later
				in.Error(action, "unsupported operation as of now", "TODO")
				return true
				// in.Save(actions[focus].Target, big.NewFloat(0))
				// in.V.Floats[in.V.Names[actions[focus].Target]].Mod(in.V.Floats[in.V.Names[o]], in.V.Floats[in.V.Names[t]])
			case BYTE:
				result := in.V.Bytes[in.V.Names[o]] / in.V.Bytes[in.V.Names[t]]
				in.Save(actions[focus].Target, result)
			}
		case "and":
			err := in.CheckDtype(action, 0, BOOL)
			if err {
				return true
			}
			err = in.CheckDtype(action, 1, BOOL)
			if err {
				return true
			}
			in.Save(action.Target, in.V.Bools[in.V.Names[string(action.Variables[0])]] && in.V.Bools[in.V.Names[string(action.Variables[1])]])
		case "error":
			e := in.CheckArgN(action, 0, 2)
			if e {
				return e
			}
			in.IgnoreErr = true
			err := in.Run(action.Target)
			switch len(action.Variables) {
			case 1:
				in.Save(string(action.Variables[0]), err)
			case 2:
				in.Save(string(action.Variables[0]), err)
				// fmt.Printf("Runtime error: %s\nLocation: line %d\nAction: %s\nType: %s\nLine:\n%s\n", message, act.Source.N+1, act.Type, etype, strings.ReplaceAll(act.Source.Source, "\r\n", "\n"))
				p := bytecode.Pair{}
				p.Ids = make(map[string]uint64)
				PairAppend(&p, in, big.NewInt(int64(in.ErrSource.N)+1), "line")
				PairAppend(&p, in, in.ErrSource.Source, "source")
				PairAppend(&p, in, action.Type, "action")
				PairAppend(&p, in, error_type, "type")
				PairAppend(&p, in, error_message, "message")
				error_type = ""
				error_message = ""
				in.Save(string(action.Variables[1]), p)
			}
			in.IgnoreErr = false
		case "or":
			err := in.CheckDtype(action, 0, BOOL)
			if err {
				return true
			}
			err = in.CheckDtype(action, 1, BOOL)
			if err {
				return true
			}
			in.Save(action.Target, in.V.Bools[in.V.Names[string(action.Variables[0])]] || in.V.Bools[in.V.Names[string(action.Variables[1])]])
		case "=":
			switch in.V.Types[in.V.Names[string(actions[focus].Variables[0])]] {
			case INT:
				in.Save(actions[focus].Target, in.V.Ints[in.V.Names[string(actions[focus].Variables[0])]])
			case FLOAT:
				in.Save(actions[focus].Target, in.V.Floats[in.V.Names[string(actions[focus].Variables[0])]])
			case STR:
				in.Save(actions[focus].Target, in.V.Strs[in.V.Names[string(actions[focus].Variables[0])]])
			case LIST:
				in.Save(actions[focus].Target, in.V.Lists[in.V.Names[string(actions[focus].Variables[0])]])
			case ARR:
				in.Save(actions[focus].Target, in.V.Arrs[in.V.Names[string(actions[focus].Variables[0])]])
			case FUNC:
				in.Save(actions[focus].Target, in.V.Funcs[in.V.Names[string(actions[focus].Variables[0])]])
			case PAIR:
				in.Save(actions[focus].Target, in.V.Pairs[in.V.Names[string(actions[focus].Variables[0])]])
			case BYTE:
				in.Save(actions[focus].Target, in.V.Bytes[in.V.Names[string(actions[focus].Variables[0])]])
			case BOOL:
				in.Save(action.Target, in.V.Bools[in.V.Names[string(actions[focus].Variables[0])]])
			case ID:
				// in.V.Names[actions[focus].Target] = in.V.Ids[in.V.Names[string(actions[focus].Variables[0])]]
				in.Save(actions[focus].Target, in.V.Ids[in.V.Names[string(actions[focus].Variables[0])]])
			case SPAN:
				in.Save(action.Target, in.V.Spans[in.V.Names[string(actions[focus].Variables[0])]])
			}
		case "&=":
			id := in.V.Names[string(actions[focus].Variables[0])]
			in.V.Names[actions[focus].Target] = id
			// u := in.V.Names[string(actions[focus].Variables[0])]
			// in.Save(actions[focus].Target, u)
		case "deep":
			//TODO: aaaaaa
			//fmt.Println("IDs:", in.V.Names[string(action.Variables[0])], in.V.Names[string(action.Variables[1])])
			in.V.Types[in.V.Names[string(action.Variables[0])]] = in.V.Types[in.V.Names[string(action.Variables[1])]]
			switch in.V.Types[in.V.Names[string(action.Variables[0])]] {
			case INT:
				in.V.Ints[in.V.Names[string(action.Variables[0])]] = in.V.Ints[in.V.Names[string(action.Variables[1])]]
			case FLOAT:
				in.V.Floats[in.V.Names[string(action.Variables[0])]] = in.V.Floats[in.V.Names[string(action.Variables[1])]]
			case STR:
				in.V.Strs[in.V.Names[string(action.Variables[0])]] = in.V.Strs[in.V.Names[string(action.Variables[1])]]
			case BYTE:
				in.V.Bytes[in.V.Names[string(action.Variables[0])]] = in.V.Bytes[in.V.Names[string(action.Variables[1])]]
			case BOOL:
				in.V.Bools[in.V.Names[string(action.Variables[0])]] = in.V.Bools[in.V.Names[string(action.Variables[1])]]
			case FUNC:
				in.V.Funcs[in.V.Names[string(action.Variables[0])]] = in.V.Funcs[in.V.Names[string(action.Variables[1])]]
			case PAIR:
				in.V.Pairs[in.V.Names[string(action.Variables[0])]] = in.V.Pairs[in.V.Names[string(action.Variables[1])]]
			case LIST:
				in.V.Lists[in.V.Names[string(action.Variables[0])]] = in.V.Lists[in.V.Names[string(action.Variables[1])]]
				//fmt.Println(in.V.Ints[in.V.Names[string(action.Variables[0])]].String())
			}
			//in.RemoveId(in.V.Names[string(action.Variables[1])])
			///in.SaveRef(in.V.Names[string(action.Variables[0])], in.GetAny(string(action.Variables[1])))
		case "GC":
			in.GC2()
			if false {
				// temp gc start
				to_del := []string{}
				for varname := range in.V.Names {
					if strings.HasPrefix(varname, "_temp_") {
						to_del = append(to_del, varname)
					}
				}
				for _, td := range to_del {
					id := in.V.Names[td]
					switch in.V.Types[id] {
					case INT:
						delete(in.V.Ints, id)
					case FLOAT:
						delete(in.V.Floats, id)
					case STR:
						delete(in.V.Strs, id)
					case ARR:
						delete(in.V.Arrs, id)
					case LIST:
						delete(in.V.Lists, id)
					case PAIR:
						delete(in.V.Pairs, id)
					case BYTE:
						delete(in.V.Bytes, id)
					case BOOL:
						delete(in.V.Bools, id)
					case ID:
						delete(in.V.Ids, id)
					}
					delete(in.V.Names, td)
				}
				// temp gc end
			}
		case "if":
			err := in.CheckDtype(actions[focus], 0, BOOL)
			if err {
				return true
			}
			b := in.V.Bools[in.V.Names[string(actions[focus].Variables[0])]]
			if b {
				result := in.Run(actions[focus].Target)
				if result {
					return true
				}
				if focus+2 < len(actions) && actions[focus+2].Type == "else" {
					focus += 2
				}
			} else if focus+2 < len(actions) && actions[focus+2].Type == "else" {
				focus += 2
				result := in.Run(actions[focus].Target)
				if result {
					return true
				}
			}
		case "while":
			err := in.CheckDtype(actions[focus], 0, BOOL)
			if err {
				return true
			}
			b := in.V.Bools[in.V.Names[string(actions[focus].Variables[0])]]
			if b {
				result := in.Run(actions[focus].Target)
				if result {
					return true
				}
				if focus+2 < len(actions) && actions[focus+2].Type == "else" {
					focus += 2
				}
				for actions[focus].Type != "while_start" {
					focus--
				}
				action = actions[focus]
			} else if focus+2 < len(actions) && actions[focus+2].Type == "else" {
				focus += 2
				result := in.Run(actions[focus].Target)
				if result {
					return true
				}
			}
		case "while_start":
		case "switch":
			in.Save("_case_", in.GetAny(string(action.Variables[0])))
			err := in.Run(action.Target)
			if err {
				return true
			}
			in.RemoveName("_case_")
		case "case":
			o, t := in.EqualizeTypes("_case_", string(actions[focus].Variables[0]))
			var err bool
			switch in.V.Types[in.V.Names[o]] {
			case INT:
				r := in.V.Ints[in.V.Names[o]].Cmp(in.V.Ints[in.V.Names[t]])
				if r == 0 {
					err = in.Run(action.Target)
					return err
				}
			case FLOAT:
				r := in.V.Floats[in.V.Names[o]].Cmp(in.V.Floats[in.V.Names[t]])
				if r == 0 {
					err = in.Run(action.Target)
					return err
				}
			case BYTE:
				if in.V.Bytes[in.V.Names[string(action.Variables[0])]] == in.V.Bytes[in.V.Names[string(action.Variables[1])]] {
					err = in.Run(action.Target)
					return err
				}
			}
		case "repeat":
			err := in.CheckArgN(action, 1, 1)
			if err {
				return err
			}
			err = in.CheckDtype(action, 0, INT)
			if err {
				return err
			}
			i := in.GetAny(string(action.Variables[0])).(*big.Int).Int64()
			for range i {
				result := in.Run(actions[focus].Target)
				if result {
					return true
				}
			}
		case "$":
			// fmt.Println(strings.TrimSpace(action.Source.Source)[1:])
			text_command := strings.TrimSpace(strings.SplitN(action.Source.Source, "$", 2)[1])
			arguments := in.Parse2(text_command)
			/*
				err := in.CheckArgN(action, 1, -1)
				if err {
					return true
				}
				var arguments []string
				if in.V.Types[in.V.Names[string(action.Variables[0])]] == LIST && len(action.Variables) == 1 {
					for n := range len(in.V.Lists[in.V.Names[string(action.Variables[0])]].Ids) {
						switch in.V.Types[in.V.Lists[in.V.Names[string(action.Variables[0])]].Ids[n]] {
						case INT:
							arguments = append(arguments, in.V.Ints[in.V.Lists[in.V.Names[string(action.Variables[0])]].Ids[n]].String())
						case FLOAT:
							arguments = append(arguments, in.V.Floats[in.V.Lists[in.V.Names[string(action.Variables[0])]].Ids[n]].String())
						case NOTH:
							arguments = append(arguments, "Nothing")
						case BYTE:
							arguments = append(arguments, fmt.Sprintf("%d", in.V.Bytes[in.V.Lists[in.V.Names[string(action.Variables[0])]].Ids[n]]))
						case STR:
							arguments = append(arguments, in.V.Strs[in.V.Lists[in.V.Names[string(action.Variables[0])]].Ids[n]])
						}
					}
				} else if in.V.Types[in.V.Names[string(action.Variables[0])]] == STR && len(action.Variables) > 0 {
					for n := range len(action.Variables) {
						switch in.V.Types[in.V.Names[string(action.Variables[n])]] {
						case INT:
							arguments = append(arguments, in.V.Ints[in.V.Names[string(action.Variables[n])]].String())
						case FLOAT:
							arguments = append(arguments, in.V.Floats[in.V.Names[string(action.Variables[n])]].String())
						case NOTH:
							arguments = append(arguments, "Nothing")
						case BYTE:
							arguments = append(arguments, fmt.Sprintf("%d", in.V.Bytes[in.V.Names[string(action.Variables[n])]]))
						case STR:
							arguments = append(arguments, in.V.Strs[in.V.Names[string(action.Variables[n])]])
						}
					}
				} else {
					in.Error(action, "Improper command format", "sys")
					return true
				}
			*/
			cmd := exec.Command(arguments[0], arguments[1:]...)
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			go_err := cmd.Run()
			if go_err != nil {
				in.Error(action, fmt.Sprintf("Error executing command: %v", go_err), "sys")
				return true
			}
		case "$$":
			text_command := strings.TrimSpace(strings.SplitN(action.Source.Source, "$", 2)[1])
			arguments := in.Parse2(text_command)
			cmd := exec.Command(arguments[0], arguments[1:]...)
			out, go_err := cmd.CombinedOutput()
			if go_err != nil {
				in.Error(action, fmt.Sprintf("Error executing command: %v", go_err), "sys")
				return true
			} else {
				in.Save(action.Target, string(out))
			}
		case "fmt":
			err := in.CheckArgN(action, 1, 1)
			if err {
				return true
			}
			err = in.CheckDtype(action, 0, STR)
			if err {
				return true
			}
			str := in.Fmt(in.V.Strs[in.V.Names[string(action.Variables[0])]])
			in.Save(action.Target, str)
		default:
			fn, ok := in.V.Funcs[in.V.Names[actions[focus].Type]]
			if !ok {
				in.Error(actions[focus], "Undeclared function!", "undeclared")
			}
			switch fn.Name {
			case "id":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return err
				}
				in.Save(action.Target, in.V.Names[string(action.Variables[0])])
				/*
					if len(actions[focus].Variables) == 1 {
						in.Save(actions[focus].Target, in.V.Names[string(actions[focus].Variables[0])])
					} else if len(actions[focus].Variables) == 2 {
						// !id one, two
						// one is id
						// two is its new value
						id := in.V.Names[string(actions[focus].Variables[0])]
						in.SaveRef(in.V.Ids[id], in.GetAny(string(actions[focus].Variables[1])))
						// TODO: verify that it's not like on the bottom:
						// in.SaveRef(id, in.GetAny(string(actions[focus].Variables[1])))
					}
				*/
			case "input":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return err
				}
				rl.SetPrompt(in.V.Strs[in.V.Names[string(action.Variables[0])]])
				str, err_ := rl.Readline()
				if err_ != nil {
					in.Error(action, "keyboard interrupt!", "interrupt")
					return true
				}
				in.Save(action.Target, str)
			case "rand":
				err := in.CheckArgN(action, 2, 2)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, FLOAT, INT)
				if err {
					return err
				}
				err = in.CheckDtype(action, 1, FLOAT, INT)
				if err {
					return err
				}
				var minimal, maximal float64
				o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
				switch in.V.Types[in.V.Names[o]] {
				case INT:
					minimal = float64(in.V.Ints[in.V.Names[o]].Int64())
					maximal = float64(in.V.Ints[in.V.Names[t]].Int64())
				case FLOAT:
					minimal, _ = in.V.Floats[in.V.Names[o]].Float64()
					maximal, _ = in.V.Floats[in.V.Names[t]].Float64()
				case BYTE:
					minimal = float64(in.V.Bytes[in.V.Names[o]])
					maximal = float64(in.V.Bytes[in.V.Names[t]])
				}
				i := rand.Float64()*(maximal-minimal) + minimal
				in.Save(action.Target, big.NewFloat(i))
			case "quit":
				in.halt = true
				os.Exit(0)
				return false
			case "list":
				if false {
					l := bytecode.List{}
					for _, variable := range actions[focus].Variables {
						id := in.V.Names[string(variable)]
						l.Ids = append(l.Ids, id)
					}
					ListUnlink(&l, in)
					in.Save(actions[focus].Target, l)
				} else {
					l := bytecode.List{}
					for _, variable := range actions[focus].Variables {
						ListAppend(&l, in, in.GetAny(string(variable)))
					}
					in.Save(actions[focus].Target, l)
				}
			case "array":
				// types := map[string]byte{"noth": 0, "int": 1, "float": 2, "str": 3, "arr": 4, "list": 5, "pair": 6, "bool": 7, "byte": 8, "func": 9, "id": 10}
				l := bytecode.Array{}
				l.Dtype = in.V.Bytes[in.V.Names[string(actions[focus].Variables[0])]]
				for _, variable := range actions[focus].Variables[1:] {
					ArrAppend(&l, in, in.GetAny(string(variable)))
				}
				in.Save(actions[focus].Target, l)
			case "pair":
				p := bytecode.Pair{}
				p.Ids = make(map[string]uint64)
				for n := 0; n < len(actions[focus].Variables); n += 2 {
					switch in.V.Types[in.V.Names[string(actions[focus].Variables[n+1])]] {
					case INT:
						PairAppend(&p, in, in.V.Ints[in.V.Names[string(actions[focus].Variables[n+1])]], in.GetAny(string(actions[focus].Variables[n])))
					case FLOAT:
						PairAppend(&p, in, in.V.Floats[in.V.Names[string(actions[focus].Variables[n+1])]], in.GetAny(string(actions[focus].Variables[n])))
					case STR:
						PairAppend(&p, in, in.V.Strs[in.V.Names[string(actions[focus].Variables[n+1])]], in.GetAny(string(actions[focus].Variables[n])))
					case PAIR:
						PairAppend(&p, in, in.V.Pairs[in.V.Names[string(actions[focus].Variables[n+1])]], in.GetAny(string(actions[focus].Variables[n])))
					case FUNC:
						PairAppend(&p, in, in.V.Funcs[in.V.Names[string(actions[focus].Variables[n+1])]], in.GetAny(string(actions[focus].Variables[n])))
					default:
						PairAppend(&p, in, in.GetAny(string(actions[focus].Variables[n+1])), in.GetAny(string(actions[focus].Variables[n])))
					}
				}
				in.Save(actions[focus].Target, p)
			case "cp":
				err := in.CheckArgN(action, 2, 2)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return err
				}
				err = in.CheckDtype(action, 1, STR)
				if err {
					return err
				}
				err2 := CopyFile(in.V.Strs[in.V.Names[string(actions[focus].Variables[0])]], in.V.Strs[in.V.Names[string(actions[focus].Variables[1])]])
				if err2 != nil {
					in.Error(action, err2.Error(), "sys")
					return true
				}
			case "rm":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return err
				}
				rm_err := os.Remove(in.V.Strs[in.V.Names[string(actions[focus].Variables[0])]])
				if rm_err != nil {
					in.Error(action, rm_err.Error(), "sys")
					return true
				}
			case "mv":
				err := in.CheckArgN(action, 2, 2)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return err
				}
				err = in.CheckDtype(action, 1, STR)
				if err {
					return err
				}
				if in.V.Strs[in.V.Names[string(actions[focus].Variables[0])]] != in.V.Strs[in.V.Names[string(actions[focus].Variables[1])]] {
					err2 := CopyFile(in.V.Strs[in.V.Names[string(actions[focus].Variables[0])]], in.V.Strs[in.V.Names[string(actions[focus].Variables[1])]])
					if err2 != nil {
						in.Error(action, err2.Error(), "sys")
						return true
					}
					rm_err := os.Remove(in.V.Strs[in.V.Names[string(actions[focus].Variables[0])]])
					if rm_err != nil {
						in.Error(action, rm_err.Error(), "sys")
						return true
					}
				}
			case "split":
				err := in.CheckArgN(action, 2, 2)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return err
				}
				err = in.CheckDtype(action, 1, STR)
				if err {
					return err
				}
				arr := strings.Split(in.V.Strs[in.V.Names[string(actions[focus].Variables[0])]], in.V.Strs[in.V.Names[string(actions[focus].Variables[1])]])
				l := bytecode.List{}
				for _, item := range arr {
					ListAppend(&l, in, item)
				}
				in.Save(action.Target, l)
			case "join":
				err := in.CheckArgN(action, 2, 2)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, LIST)
				if err {
					return err
				}
				err = in.CheckDtype(action, 1, STR)
				if err {
					return err
				}
				arr := []string{}
				l := in.V.Lists[in.V.Names[string(actions[focus].Variables[0])]]
				for _, id := range l.Ids {
					if in.V.Types[id] != STR {
						in.Error(action, "join error: non-str element!", "arg_type")
						return true
					} else {
						arr = append(arr, in.V.Strs[id])
					}
				}
				in.Save(action.Target, strings.Join(arr, in.V.Strs[in.V.Names[string(actions[focus].Variables[1])]]))
			case "chdir":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return err
				}
				ch_err := os.Chdir(in.V.Strs[in.V.Names[string(actions[focus].Variables[0])]])
				if ch_err != nil {
					in.Error(action, ch_err.Error(), "sys")
					return true
				}
			case "has":
				err := in.CheckArgN(action, 2, 2)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, LIST, STR)
				if err {
					return err
				}
				switch in.V.Types[in.V.Names[string(action.Variables[0])]] {
				case LIST:
					l := in.GetAny(string(action.Variables[0])).(bytecode.List)
					in.Save(action.Target, false)
					for n := range len(l.Ids) {
						in.Save("_temp_c", in.GetAnyRef(l.Ids[n]))
						o, t := in.EqualizeTypes("_temp_c", string(actions[focus].Variables[1]))
						switch in.V.Types[in.V.Names[o]] {
						case INT:
							r := in.V.Ints[in.V.Names[o]].Cmp(in.V.Ints[in.V.Names[t]])
							if r == 0 {
								in.Save(actions[focus].Target, true)
								break
							}
						case FLOAT:
							r := in.V.Floats[in.V.Names[o]].Cmp(in.V.Floats[in.V.Names[t]])
							if r == 0 {
								in.Save(actions[focus].Target, true)
								break
							}
						case BYTE:
							if in.V.Bytes[in.V.Names[o]] == in.V.Bytes[in.V.Names[t]] {
								in.Save(action.Target, true)
								break
							}
						case STR:
							if in.V.Strs[in.V.Names[o]] == in.V.Strs[in.V.Names[t]] {
								in.Save(action.Target, true)
								break
							}
						}
					}
				case STR:
					err = in.CheckDtype(action, 1, STR)
					if err {
						return err
					}
					in.Save(action.Target, strings.Contains(in.V.Strs[in.V.Names[string(action.Variables[0])]], in.V.Strs[in.V.Names[string(action.Variables[1])]]))
				}
			case "index":
				err := in.CheckArgN(action, 2, 2)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, LIST)
				if err {
					return err
				}
				l := in.GetAny(string(action.Variables[0])).(bytecode.List)
				in.Save(action.Target, big.NewInt(int64(-1)))
				for n := range len(l.Ids) {
					in.Save("_temp_c", in.GetAnyRef(l.Ids[n]))
					o, t := in.EqualizeTypes("_temp_c", string(actions[focus].Variables[1]))
					switch in.V.Types[in.V.Names[o]] {
					case INT:
						r := in.V.Ints[in.V.Names[o]].Cmp(in.V.Ints[in.V.Names[t]])
						if r == 0 {
							in.Save(actions[focus].Target, big.NewInt(int64(n)))
							break
						}
					case FLOAT:
						r := in.V.Floats[in.V.Names[o]].Cmp(in.V.Floats[in.V.Names[t]])
						if r == 0 {
							in.Save(actions[focus].Target, big.NewInt(int64(n)))
							break
						}
					case BYTE:
						if in.V.Bytes[in.V.Names[o]] == in.V.Bytes[in.V.Names[t]] {
							in.Save(action.Target, big.NewInt(int64(n)))
							break
						}
					case STR:
						if in.V.Strs[in.V.Names[o]] == in.V.Strs[in.V.Names[t]] {
							in.Save(action.Target, big.NewInt(int64(n)))
							break
						}
					}
				}
			case "system":
				err := in.CheckArgN(actions[focus], 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(actions[focus], 0, STR)
				if err {
					return true
				}
				switch in.V.Strs[in.V.Names[string(actions[focus].Variables[0])]] {
				case "os":
					in.Save(actions[focus].Target, runtime.GOOS)
				case "version":
					in.Save(actions[focus].Target, "4.0.8")
				case "args":
					l := bytecode.List{}
					for _, arg := range os.Args {
						ListAppend(&l, in, arg)
					}
					in.Save(actions[focus].Target, l)
				case "cwd":
					wd, err := os.Getwd()
					if err != nil {
						in.Error(action, err.Error(), "sys")
					}
					in.Save(actions[focus].Target, wd)
				}
			case "pop":
				err := in.CheckArgN(actions[focus], 2, 2)
				if err {
					return true
				}
				err = in.CheckDtype(actions[focus], 0, LIST, ARR, PAIR)
				if err {
					return true
				}
				/*
					err = in.CheckDtype(actions[focus], 1, INT, STR)
					if err {
						return true
					}
				*/
				switch in.V.Types[in.V.Names[string(action.Variables[0])]] {
				case LIST:
					ind := int(in.V.Ints[in.V.Names[string(action.Variables[1])]].Int64())
					l := in.V.Lists[in.V.Names[string(action.Variables[0])]]
					if ind < 0 {
						ind += len(l.Ids)
					}
					in.Save(action.Target, in.GetAnyRef(l.Ids[ind]))
					l.Ids = append(l.Ids[:ind], l.Ids[ind+1:]...)
					in.Save(string(action.Variables[0]), l)
				case ARR:
				case PAIR:
					k := PairKey(in, in.GetAny(string(action.Variables[1])))
					p := in.V.Pairs[in.V.Names[string(action.Variables[0])]]
					in.Save(action.Target, in.GetAnyRef(p.Ids[k]))
					delete(p.Ids, k)
					in.Save(string(action.Variables[0]), p)
				}
			case "cti":
				err := in.CheckArgN(actions[focus], 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(actions[focus], 0, STR)
				if err {
					return true
				}
				str := in.GetAny(string(action.Variables[0])).(string)
				in.Save(action.Target, big.NewInt(int64(str[0])))
			case "itc":
				err := in.CheckArgN(actions[focus], 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(actions[focus], 0, INT)
				if err {
					return true
				}
				i := in.GetAny(string(action.Variables[0])).(*big.Int)
				in.Save(action.Target, string(rune(i.Int64())))
			case "print":
				for n := 0; n < len(actions[focus].Variables); n++ {
					switch in.V.Types[in.V.Names[string(actions[focus].Variables[n])]] {
					case NOTH:
						fmt.Print("Nothing")
					case INT:
						fmt.Printf("%s", in.V.Ints[in.V.Names[string(actions[focus].Variables[n])]].String())
					case BYTE:
						fmt.Printf("b.%d", in.V.Bytes[in.V.Names[string(actions[focus].Variables[n])]])
					case STR:
						fmt.Printf("%s", in.V.Strs[in.V.Names[string(actions[focus].Variables[n])]])
					case FLOAT:
						fmt.Printf("%s", in.V.Floats[in.V.Names[string(actions[focus].Variables[n])]].String())
					case FUNC:
						fmt.Printf("func.%s", in.V.Funcs[in.V.Names[string(actions[focus].Variables[n])]].Name)
					case LIST:
						l := in.V.Lists[in.V.Names[string(actions[focus].Variables[n])]]
						fmt.Print(ListString(&l, in))
					case ARR:
						l := in.V.Arrs[in.V.Names[string(actions[focus].Variables[n])]]
						fmt.Print(l.String())
					case PAIR:
						p := in.V.Pairs[in.V.Names[string(actions[focus].Variables[n])]]
						fmt.Print(PairString(&p, in))
					case ID:
						fmt.Printf("%d", in.V.Ids[in.V.Names[string(actions[focus].Variables[n])]])
					case BOOL:
						fmt.Print(in.V.Bools[in.V.Names[string(actions[focus].Variables[n])]])
					case SPAN:
						am := in.V.Spans[in.V.Names[string(actions[focus].Variables[n])]]
						fmt.Print(ArrMString(&am, in))
					}
					if n < len(actions[focus].Variables)-1 {
						fmt.Print(" ")
					} else {
						fmt.Println()
					}
				}
			case "ternary":
				err := in.CheckArgN(actions[focus], 3, 3)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, BOOL)
				if err {
					return true
				}
				b := in.V.Bools[in.V.Names[string(action.Variables[0])]]
				if b {
					in.Save(action.Target, in.GetAny(string(action.Variables[1])))
				} else {
					in.Save(action.Target, in.GetAny(string(action.Variables[2])))
				}
			case "type":
				err := in.CheckArgN(actions[focus], 1, 1)
				if err {
					return true
				}
				dtype := map[byte]string{0: "noth", 1: "int", 2: "float", 3: "str", 4: "arr", 5: "list", 6: "pair", 7: "bool", 8: "byte", 9: "func", 10: "id"}
				in.Save(actions[focus].Target, dtype[in.V.Types[in.V.Names[string(actions[focus].Variables[0])]]])
			case "convert":
				err := in.CheckArgN(actions[focus], 2, 2)
				if err {
					return true
				}
				srcName := string(actions[focus].Variables[0])
				exName := string(actions[focus].Variables[1])
				srcId := in.V.Names[srcName]
				exId := in.V.Names[exName]
				srcType := in.V.Types[srcId]
				exType := in.V.Types[exId]

				switch exType {
				case INT:
					switch srcType {
					case STR:
						s := in.V.Strs[srcId]
						b := big.NewInt(0)
						b.SetString(s, 10)
						in.Save(actions[focus].Target, b)
					case FLOAT:
						f := in.V.Floats[srcId]
						i, _ := f.Int(nil)
						in.Save(actions[focus].Target, i)
					case BOOL:
						in.Save(actions[focus].Target, ternary(in.V.Bools[srcId], big.NewInt(1), big.NewInt(0)))
					case BYTE:
						in.Save(actions[focus].Target, big.NewInt(int64(in.V.Bytes[srcId])))
					}

				case FLOAT:
					switch srcType {
					case STR:
						s := in.V.Strs[srcId]
						f := big.NewFloat(0)
						f.SetString(s)
						in.Save(actions[focus].Target, f)
					case INT:
						i := in.V.Ints[srcId]
						f := new(big.Float).SetInt(i)
						in.Save(actions[focus].Target, f)
					case BOOL:
						in.Save(actions[focus].Target, ternary(in.V.Bools[srcId], big.NewFloat(1), big.NewFloat(0)))
					case BYTE:
						in.Save(actions[focus].Target, big.NewFloat(float64(in.V.Bytes[srcId])))
					}

				case STR:
					switch srcType {
					case INT:
						in.Save(actions[focus].Target, in.V.Ints[srcId].String())
					case FLOAT:
						in.Save(actions[focus].Target, in.V.Floats[srcId].String())
					case STR:
						in.Save(actions[focus].Target, in.V.Strs[srcId])
					case LIST:
						l := in.V.Lists[srcId]
						in.Save(actions[focus].Target, ListString(&l, in))
					case ARR:
						a := in.V.Arrs[srcId]
						if a.Dtype == BYTE {
							str := ""
							for _, id := range a.Bytes {
								str += string(rune(id))
							}
							in.Save(actions[focus].Target, str)
						}
					case PAIR:
						p := in.V.Pairs[srcId]
						in.Save(actions[focus].Target, PairString(&p, in))
					case BOOL:
						in.Save(actions[focus].Target, ternary(in.V.Bools[srcId], "true", "false"))
					case BYTE:
						in.Save(actions[focus].Target, fmt.Sprintf("b.%d", in.V.Bytes[srcId]))
					}

				case LIST:
					switch srcType {
					case STR:
						in.StrToList(srcName, actions[focus].Target)
					case ARR:
						a := in.V.Arrs[srcId]
						l := bytecode.List{}
						for _, id := range a.Ids {
							ListAppend(&l, in, in.GetAnyRef(id))
						}
						in.Save(actions[focus].Target, l)
					}

				case ARR:
					a := in.V.Arrs[exId]
					if a.Dtype != BYTE {
						in.Error(actions[focus], "only byte-arrays supported for conversion", "arg_type")
						return true
					}
					switch srcType {
					case STR:
						str := in.V.Strs[srcId]
						na := bytecode.Array{Dtype: BYTE}
						for _, r := range str {
							ArrAppend(&na, in, byte(r))
						}
						in.Save(actions[focus].Target, na)
					case LIST:
						l := in.V.Lists[srcId]
						na := bytecode.Array{Dtype: NOTH}
						for _, id := range l.Ids {
							val := in.GetAnyRef(id)
							errStr := na.Append(val) //ArrAppend(&na, in, val)
							if errStr != nil {
								in.Error(actions[focus], errStr.Error(), "arg_type")
								return true
							}
						}
						in.Save(actions[focus].Target, na)
					}

				case BYTE:
					switch srcType {
					case INT:
						in.Save(actions[focus].Target, byte(in.V.Ints[srcId].Int64()))
					case FLOAT:
						f, _ := in.V.Floats[srcId].Int(nil)
						in.Save(actions[focus].Target, byte(f.Int64()))
					}

				default:
					in.Error(actions[focus], "unsupported conversion target type", "arg_type")
					return true
				}
			case "replace":
				err := in.CheckArgN(action, 3, 3)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return err
				}
				err = in.CheckDtype(action, 1, STR)
				if err {
					return err
				}
				err = in.CheckDtype(action, 2, STR)
				if err {
					return err
				}
				o, t, h := in.V.Strs[in.V.Names[string(action.Variables[0])]], in.V.Strs[in.V.Names[string(action.Variables[1])]], in.V.Strs[in.V.Names[string(action.Variables[2])]]
				in.Save(action.Target, strings.ReplaceAll(o, t, h))
			case "re_find":
				err := in.CheckArgN(action, 2, 2)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return err
				}
				err = in.CheckDtype(action, 1, STR)
				if err {
					return err
				}
				l := bytecode.List{}
				r, r_err := regexp.Compile(in.V.Strs[in.V.Names[string(action.Variables[0])]])
				if r_err != nil {
					in.Error(action, r_err.Error(), "regex")
				}
				for _, item := range r.FindAllString(in.V.Strs[in.V.Names[string(action.Variables[1])]], -1) {
					ListAppend(&l, in, item)
				}
				in.Save(action.Target, l)
			case "re_match":
				err := in.CheckArgN(action, 2, 2)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return err
				}
				err = in.CheckDtype(action, 1, STR)
				if err {
					return err
				}
				r, r_err := regexp.Compile(in.V.Strs[in.V.Names[string(action.Variables[0])]])
				if r_err != nil {
					in.Error(action, r_err.Error(), "regex")
				}
				in.Save(action.Target, r.MatchString(in.V.Strs[in.V.Names[string(action.Variables[1])]]))
			case "rget":
				err := in.CheckArgN(actions[focus], 1, 1)
				if err {
					return true
				}
				if in.CheckDtype(actions[focus], 0, STR) {
					return true
				}
				url := in.V.Strs[in.V.Names[string(actions[focus].Variables[0])]]
				resp, err2 := http.Get(url)
				if err2 != nil {
					in.Error(actions[focus], err2.Error(), "sys")
					return true
				}

				body, err3 := io.ReadAll(resp.Body)
				if err3 != nil {
					in.Error(actions[focus], err3.Error(), "sys")
					return true
				}
				pnew := bytecode.Pair{}
				pnew.Ids = make(map[string]uint64)
				PairAppend(&pnew, in, big.NewInt(int64(resp.StatusCode)), "code")
				PairAppend(&pnew, in, string(body), "body")
				//println(PairString(&pnew, in))
				in.Save(actions[focus].Target, pnew)
				resp.Body.Close()
			case "rpost":
				err := in.CheckArgN(actions[focus], 2, 2)
				if err {
					return true
				}
				if !in.CheckDtype(actions[focus], 0, STR) || !in.CheckDtype(actions[focus], 1, PAIR) {
					return true
				}
				url := in.V.Strs[in.V.Names[string(actions[focus].Variables[0])]]
				pair := in.V.Pairs[in.V.Names[string(actions[focus].Variables[1])]]
				jsonStr := PairString(&pair, in)
				resp, err2 := http.Post(url, "application/json", bytes.NewBuffer([]byte(jsonStr)))
				if err2 != nil {
					in.Error(actions[focus], err2.Error(), "sys")
					return true
				}

				body, err3 := io.ReadAll(resp.Body)
				if err3 != nil {
					in.Error(actions[focus], err3.Error(), "sys")
					return true
				}
				pnew := bytecode.Pair{}
				pnew.Ids = make(map[string]uint64)
				PairAppend(&pnew, in, big.NewInt(int64(resp.StatusCode)), "code")
				PairAppend(&pnew, in, string(body), "body")
				in.Save(actions[focus].Target, pnew)
				resp.Body.Close()
			case "fmt":
				err := in.CheckArgN(actions[focus], 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(actions[focus], 0, STR)
				if err {
					return true
				}
				in.Save(action.Target, in.ParseStr(in.GetAny(string(action.Variables[0])).(string)))
			case "read":
				err := in.CheckArgN(actions[focus], 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(actions[focus], 0, STR)
				if err {
					return true
				}
				b, berr := os.ReadFile(in.V.Strs[in.V.Names[string(action.Variables[0])]])
				if berr != nil {
					in.Error(action, berr.Error(), "file")
				}
				a := bytecode.Array{}
				a.Bytes = append(a.Bytes, b...)
				a.Dtype = BYTE
				in.Save(action.Target, a)
			case "write":
				if is_safe {
					in.Error(action, "cannot write to files when in safe mode!", "permission")
					return true
				}
				err := in.CheckArgN(actions[focus], 2, 2)
				if err {
					return true
				}
				err = in.CheckDtype(actions[focus], 0, STR)
				if err {
					return true
				}
				err = in.CheckDtype(actions[focus], 1, ARR)
				if err {
					return true
				}
				a := in.V.Arrs[in.V.Names[string(action.Variables[1])]]
				bobj := a.Bytes
				oserr := os.WriteFile(in.V.Strs[in.V.Names[string(action.Variables[0])]], bobj, 0644)
				if oserr != nil {
					in.Error(action, oserr.Error(), "sys")
					return true
				}
			case "len":
				err := in.CheckArgN(actions[focus], 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(actions[focus], 0, LIST, STR, ARR, PAIR)
				if err {
					return true
				}
				switch in.V.Types[in.V.Names[string(actions[focus].Variables[0])]] {
				case STR:
					i := int64(0)
					for range in.V.Strs[in.V.Names[string(actions[focus].Variables[0])]] {
						i++
					}
					length := big.NewInt(i)
					in.Save(actions[focus].Target, length)
				case LIST:
					in.Save(actions[focus].Target, big.NewInt(int64(len(in.V.Lists[in.V.Names[string(actions[focus].Variables[0])]].Ids))))
				case PAIR:
					in.Save(actions[focus].Target, big.NewInt(int64(len(in.V.Pairs[in.V.Names[string(actions[focus].Variables[0])]].Ids))))
				case ARR:
					in.Save(actions[focus].Target, big.NewInt(int64(len(in.V.Arrs[in.V.Names[string(actions[focus].Variables[0])]].Ids))))
				}
			case "arrm":
				am := bytecode.Span{}
				for _, v_name := range action.Variables {
					name := string(v_name)
					a := in.GetAny(name)
					err := ArrMAppend(&am, in, a)
					if err != "" {
						in.Error(action, err, "arrm")
						return true
					}
				}
				println(ArrMString(&am, in))
				in.Save(action.Target, am)
			case "range":
				err := in.CheckArgN(actions[focus], 1, 3)
				if err {
					return true
				}
				l := bytecode.List{}
				var start, end, step int
				switch len(action.Variables) {
				case 1:
					err = in.CheckDtype(actions[focus], 0, INT)
					if err {
						return true
					}
					start, end, step = 0, int(in.V.Ints[in.V.Names[string(action.Variables[0])]].Int64()), 1
				case 2:
					err = in.CheckDtype(actions[focus], 0, INT)
					if err {
						return true
					}
					err = in.CheckDtype(actions[focus], 1, INT)
					if err {
						return true
					}
					start, end, step = int(in.V.Ints[in.V.Names[string(action.Variables[0])]].Int64()), int(in.V.Ints[in.V.Names[string(action.Variables[1])]].Int64()), 1
				case 3:
					start, end, step = int(in.V.Ints[in.V.Names[string(action.Variables[0])]].Int64()), int(in.V.Ints[in.V.Names[string(action.Variables[1])]].Int64()), int(in.V.Ints[in.V.Names[string(action.Variables[2])]].Int64())
				}
				for n := start; n < end; n += step {
					ListAppend(&l, in, big.NewInt(int64(n)))
				}
				in.Save(action.Target, l)
			case "glob":
				err := in.CheckArgN(actions[focus], 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(actions[focus], 0, STR)
				if err {
					return true
				}
				arr, err2 := filepath.Glob(in.V.Strs[in.V.Names[string(action.Variables[0])]])
				if err2 != nil {
					in.Error(action, err2.Error(), "sys")
				}
				l := bytecode.List{}
				for _, file := range arr {
					ListAppend(&l, in, file)
				}
				in.Save(action.Target, l)
			case "env":
				err := in.CheckArgN(action, 1, 2)
				if err {
					return err
				}
				if len(action.Variables) == 2 {
					err = in.CheckDtype(action, 0, STR)
					if err {
						return err
					}
					err = in.CheckDtype(action, 1, STR)
					if err {
						return err
					}
					os.Setenv(in.V.Strs[in.V.Names[string(action.Variables[0])]], in.V.Strs[in.V.Names[string(action.Variables[1])]])
				} else {
					err = in.CheckDtype(action, 0, STR)
					if err {
						return err
					}
					in.Save(action.Target, os.Getenv(in.V.Strs[in.V.Names[string(action.Variables[0])]]))
				}
			case "append":
				err := in.CheckArgN(action, 2, 3)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, LIST, ARR, PAIR)
				if err {
					return true
				}
				if in.V.Types[in.V.Names[string(action.Variables[0])]] == LIST {
					err = in.CheckArgN(action, 2, 2)
					if err {
						return true
					}
					// in.V.Lists[in.V.Names[string(action.Variables[0])]].Ids = append(in.V.Lists[in.V.Names[string(action.Variables[0])]].Ids, in.V.Names[string(action.Variables[1])])
					// TODO: figure this shit out
					// println("test")
					l := in.V.Lists[in.V.Names[string(action.Variables[0])]]
					ListAppend(&l, in, in.GetAny(string(action.Variables[1])))
					// println(len(l.Ids))
					// ListUnlink(&l, in)
					in.Save(action.Target, l)
				} else if in.V.Types[in.V.Names[string(action.Variables[0])]] == ARR {
					err = in.CheckArgN(action, 2, 2)
					if err {
						return true
					}

					a := in.V.Arrs[in.V.Names[string(action.Variables[0])]]
					result := ArrAppend(&a, in, in.GetAny(string(action.Variables[1])))
					if result != "" {
						in.Error(action, result, "type")
					}
					in.Save(action.Target, a)
				} else if in.V.Types[in.V.Names[string(action.Variables[0])]] == PAIR {
					err = in.CheckArgN(action, 3, 3)
					if err {
						return true
					}

					a := in.V.Pairs[in.V.Names[string(action.Variables[0])]]
					PairAppend(&a, in, in.GetAny(string(action.Variables[2])), in.GetAny(string(action.Variables[1])))
					in.Save(action.Target, a)
				}
			default:
				if fn.Node != "" {
					// user functions start
					f_in := Interpreter{}
					f_in.Copy(in)
					for n, fn_arg := range fn.Vars {
						fn_arg_str := string(fn_arg)
						if in.V.Types[in.V.Names[string(action.Variables[n])]] == PAIR {
							p := f_in.CopyPair(string(action.Variables[n]), in)
							f_in.Save(fn_arg_str, p)
						} else if in.V.Types[in.V.Names[string(action.Variables[n])]] == LIST {
							l := f_in.CopyList(string(action.Variables[n]), in)
							f_in.Save(fn_arg_str, l)
						} else {
							f_in.Save(fn_arg_str, in.GetAny(string(action.Variables[n])))
						}
					}
					err := f_in.Run(fn.Node)
					in.ErrSource = f_in.ErrSource
					if err {
						return err
					}
					if f_in.V.Types[f_in.V.Names["_return_"]] == LIST {
						l := in.CopyList("_return_", &f_in)
						in.Save(action.Target, l)
					} else if f_in.V.Types[f_in.V.Names["_return_"]] == PAIR {
						l := in.CopyPair("_return_", &f_in)
						in.Save(action.Target, l)
					} else {
						in.Save(action.Target, f_in.GetAny("_return_"))
					}
					f_in.Destroy()
					/*
						if val, ok := f_in.v.Types["_return_"]; ok {
							in.v.Types[action.Target] = val
							switch val {
							case INT:
								in.v.Ints[action.Target] = f_in.v.Ints["_return_"]
							case FLOAT:
								in.v.Floats[action.Target] = f_in.v.Floats["_return_"]
							case STR:
								in.v.Strs[action.Target] = f_in.v.Strs["_return_"]
							case ARR:
								in.v.Arrs[action.Target] = f_in.v.Arrs["_return_"]
							case LIST:
								in.v.Lists[action.Target] = f_in.v.Lists["_return_"]
							case BYTE:
								in.v.Bytes[action.Target] = f_in.v.Bytes["_return_"]
							case PAIR:
								in.v.Pairs[action.Target] = f_in.v.Pairs["_return_"]
							case FUNC:
								in.v.Funs[action.Target] = f_in.v.Funs["_return_"]
							case BOOL:
								in.v.Bools[action.Target] = f_in.v.Bools["_return_"]
							}
						}
					*/
					// user functions end
				} else {
					in.Error(actions[focus], "Undeclared function!", "undeclared")
				}
			}
		}
		focus++
	}
	return false
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

func (in *Interpreter) Copy(og *Interpreter) {
	in.IgnoreErr = og.IgnoreErr
	in.Code = og.Code
	in.File = og.File
	in.V = &bytecode.Vars{}
	in.V.Bools = make(map[uint64]bool)
	in.V.Bytes = make(map[uint64]byte)
	in.V.Floats = make(map[uint64]*big.Float)
	in.V.Funcs = make(map[uint64]bytecode.Function)
	in.V.Ints = make(map[uint64]*big.Int)
	in.V.Names = make(map[string]uint64)
	in.V.Strs = make(map[uint64]string)
	in.V.Types = make(map[uint64]byte)
	in.V.Ids = make(map[uint64]uint64)
	in.V.Lists = make(map[uint64]bytecode.List)
	in.V.Arrs = make(map[uint64]bytecode.Array)
	in.V.Pairs = make(map[uint64]bytecode.Pair)
	for _, fn := range bytecode.GenerateFuns() {
		in.Save(fn.Name, fn)
	}
	for key := range og.V.Names {
		switch og.V.Types[og.V.Names[key]] {
		case INT:
			in.Save(key, og.V.Ints[og.V.Names[key]])
		case FLOAT:
			in.Save(key, og.V.Floats[og.V.Names[key]])
		case STR:
			in.Save(key, og.V.Strs[og.V.Names[key]])
		case BOOL:
			in.Save(key, og.V.Bools[og.V.Names[key]])
		case BYTE:
			in.Save(key, og.V.Bytes[og.V.Names[key]])
		case FUNC:
			in.Save(key, og.V.Funcs[og.V.Names[key]])
		}
	}
}

func NewInterpreter(code, file string) Interpreter {
	in := Interpreter{}
	in.Code = bytecode.GetCode(code)
	in.File = &file
	in.V = &bytecode.Vars{}
	in.V.Bools = make(map[uint64]bool)
	in.V.Bytes = make(map[uint64]byte)
	in.V.Floats = make(map[uint64]*big.Float)
	in.V.Funcs = make(map[uint64]bytecode.Function)
	in.V.Ints = make(map[uint64]*big.Int)
	in.V.Names = make(map[string]uint64)
	in.V.Strs = make(map[uint64]string)
	in.V.Types = make(map[uint64]byte)
	in.V.Ids = make(map[uint64]uint64)
	in.V.Lists = make(map[uint64]bytecode.List)
	in.V.Arrs = make(map[uint64]bytecode.Array)
	in.V.Pairs = make(map[uint64]bytecode.Pair)
	in.V.Spans = make(map[uint64]bytecode.Span)
	for _, fn := range bytecode.GenerateFuns() {
		in.Save(fn.Name, fn)
	}
	return in
}

func (in *Interpreter) Compile(code, fname string) {
	in2 := Interpreter{}
	in2.Code = bytecode.GetCode(code)
	in2.File = &fname
	for key := range in2.Code {
		in.Code[key] = in2.Code[key]
		delete(in2.Code, key)
	}
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
