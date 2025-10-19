package inter

import (
	"fmt"
	"math"
	"math/big"
	"minimum/bytecode"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

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

type Entry struct {
	Type  byte
	Index int
}

type Vars struct {
	Names   map[string]int // the name points to a slot number
	Slots   []Entry        // each entry tells the type of data and its real address
	Ints    []*big.Int
	Floats  []*big.Float
	Strs    []string
	Bools   []bool
	Bytes   []byte
	Funcs   []*bytecode.Function
	Ids     []uint64
	Arrs    []bytecode.Array
	Spans   []bytecode.Span
	Lists   []bytecode.List
	Pairs   []bytecode.Pair
	gcCycle uint16
	gcMax   uint16
}

type Interpreter struct {
	V         *Vars
	Code      map[string][]bytecode.Action
	Parent    *Interpreter
	File      *string
	halt      bool
	SwitchId  string
	IgnoreErr bool
	ErrSource *bytecode.SourceLine
}

func TypeToByte(a any) byte {
	switch a.(type) {
	case *big.Int:
		return INT
	case *big.Float:
		return FLOAT
	case string:
		return STR
	case byte:
		return BYTE
	case bool:
		return BOOL
	case uint64:
		return ID
	case bytecode.List:
		return LIST
	case bytecode.Array:
		return ARR
	case bytecode.Pair:
		return PAIR
	case *bytecode.Function:
		return FUNC
	case bytecode.Span:
		return SPAN
	}
	return NOTH
}

func (in *Interpreter) Save(name string, v any) {
	if old_id, ok := in.V.Names[name]; ok {
		if TypeToByte(v) == in.V.Slots[old_id].Type {
			// value reassignment
			switch in.V.Slots[old_id].Type {
			case INT:
				in.V.Ints[in.V.Slots[old_id].Index] = v.(*big.Int)
			case FLOAT:
				in.V.Floats[in.V.Slots[old_id].Index] = v.(*big.Float)
			case STR:
				in.V.Strs[in.V.Slots[old_id].Index] = v.(string)
			case BYTE:
				in.V.Bytes[in.V.Slots[old_id].Index] = v.(byte)
			case BOOL:
				in.V.Bools[in.V.Slots[old_id].Index] = v.(bool)
			case LIST:
				in.V.Lists[in.V.Slots[old_id].Index] = v.(bytecode.List)
			case ARR:
				in.V.Arrs[in.V.Slots[old_id].Index] = v.(bytecode.Array)
			case SPAN:
				in.V.Spans[in.V.Slots[old_id].Index] = v.(bytecode.Span)
			case FUNC:
				in.V.Funcs[in.V.Slots[old_id].Index] = v.(*bytecode.Function)
			}
			return
		}
		// "else" will be handled by GC
	}
	entry := Entry{TypeToByte(v), 0}
	switch val := v.(type) {
	case *big.Int:
		entry.Index = len(in.V.Ints)
		in.V.Ints = append(in.V.Ints, val)
	case *big.Float:
		entry.Index = len(in.V.Floats)
		in.V.Floats = append(in.V.Floats, val)
	case string:
		entry.Index = len(in.V.Strs)
		in.V.Strs = append(in.V.Strs, val)
	case byte:
		entry.Index = len(in.V.Bytes)
		in.V.Bytes = append(in.V.Bytes, val)
	case bool:
		entry.Index = len(in.V.Bools)
		in.V.Bools = append(in.V.Bools, val)
	case uint64:
		entry.Index = len(in.V.Ids)
		in.V.Ids = append(in.V.Ids, val)
	case bytecode.List:
		entry.Index = len(in.V.Lists)
		in.V.Lists = append(in.V.Lists, val)
	case bytecode.Array:
		entry.Index = len(in.V.Arrs)
		in.V.Arrs = append(in.V.Arrs, val)
	case bytecode.Pair:
		entry.Index = len(in.V.Pairs)
		in.V.Pairs = append(in.V.Pairs, val)
	case *bytecode.Function:
		entry.Index = len(in.V.Funcs)
		in.V.Funcs = append(in.V.Funcs, val)
	case bytecode.Span:
		entry.Index = len(in.V.Spans)
		in.V.Spans = append(in.V.Spans, val)
	}
	in.V.Names[name] = len(in.V.Slots)
	in.V.Slots = append(in.V.Slots, entry)
}

func (in *Interpreter) SaveRef(old_id uint64, v any) {
	if TypeToByte(v) == in.V.Slots[old_id].Type {
		// value reassignment
		switch in.V.Slots[old_id].Type {
		case INT:
			in.V.Ints[in.V.Slots[old_id].Index] = v.(*big.Int)
		case FLOAT:
			in.V.Floats[in.V.Slots[old_id].Index] = v.(*big.Float)
		case STR:
			in.V.Strs[in.V.Slots[old_id].Index] = v.(string)
		case BYTE:
			in.V.Bytes[in.V.Slots[old_id].Index] = v.(byte)
		case BOOL:
			in.V.Bools[in.V.Slots[old_id].Index] = v.(bool)
		case LIST:
			in.V.Lists[in.V.Slots[old_id].Index] = v.(bytecode.List)
		case ARR:
			in.V.Arrs[in.V.Slots[old_id].Index] = v.(bytecode.Array)
		case SPAN:
			in.V.Spans[in.V.Slots[old_id].Index] = v.(bytecode.Span)
		case FUNC:
			in.V.Funcs[in.V.Slots[old_id].Index] = v.(*bytecode.Function)
		}
		return
	}
	entry := Entry{TypeToByte(v), 0}
	switch val := v.(type) {
	case *big.Int:
		entry.Index = len(in.V.Ints)
		in.V.Ints = append(in.V.Ints, val)
	case *big.Float:
		entry.Index = len(in.V.Floats)
		in.V.Floats = append(in.V.Floats, val)
	case string:
		entry.Index = len(in.V.Strs)
		in.V.Strs = append(in.V.Strs, val)
	case byte:
		entry.Index = len(in.V.Bytes)
		in.V.Bytes = append(in.V.Bytes, val)
	case bool:
		entry.Index = len(in.V.Bools)
		in.V.Bools = append(in.V.Bools, val)
	case uint64:
		entry.Index = len(in.V.Ids)
		in.V.Ids = append(in.V.Ids, val)
	case bytecode.List:
		entry.Index = len(in.V.Lists)
		in.V.Lists = append(in.V.Lists, val)
	case bytecode.Array:
		entry.Index = len(in.V.Arrs)
		in.V.Arrs = append(in.V.Arrs, val)
	case bytecode.Pair:
		entry.Index = len(in.V.Pairs)
		in.V.Pairs = append(in.V.Pairs, val)
	case *bytecode.Function:
		entry.Index = len(in.V.Funcs)
		in.V.Funcs = append(in.V.Funcs, val)
	case bytecode.Span:
		entry.Index = len(in.V.Spans)
		in.V.Spans = append(in.V.Spans, val)
	}
	// in.V.Names[name] = len(in.V.Slots)
	in.V.Slots = append(in.V.Slots, entry)
}

func (in *Interpreter) SaveRefNew(v any) uint64 {
	entry := Entry{TypeToByte(v), 0}
	var new_ref uint64
	switch val := v.(type) {
	case *big.Int:
		entry.Index = len(in.V.Ints)
		in.V.Ints = append(in.V.Ints, val)
	case *big.Float:
		entry.Index = len(in.V.Floats)
		in.V.Floats = append(in.V.Floats, val)
	case string:
		entry.Index = len(in.V.Strs)
		in.V.Strs = append(in.V.Strs, val)
	case byte:
		entry.Index = len(in.V.Bytes)
		in.V.Bytes = append(in.V.Bytes, val)
	case bool:
		entry.Index = len(in.V.Bools)
		in.V.Bools = append(in.V.Bools, val)
	case uint64:
		entry.Index = len(in.V.Ids)
		in.V.Ids = append(in.V.Ids, val)
	case bytecode.List:
		entry.Index = len(in.V.Lists)
		in.V.Lists = append(in.V.Lists, val)
	case bytecode.Array:
		entry.Index = len(in.V.Arrs)
		in.V.Arrs = append(in.V.Arrs, val)
	case bytecode.Pair:
		entry.Index = len(in.V.Pairs)
		in.V.Pairs = append(in.V.Pairs, val)
	case *bytecode.Function:
		entry.Index = len(in.V.Funcs)
		in.V.Funcs = append(in.V.Funcs, val)
	case bytecode.Span:
		entry.Index = len(in.V.Spans)
		in.V.Spans = append(in.V.Spans, val)
	}
	new_ref = uint64(len(in.V.Slots))
	in.V.Slots = append(in.V.Slots, entry)
	return new_ref
}

func (in *Interpreter) GetAny(var_name string) any {
	ind := in.V.Slots[in.V.Names[var_name]].Index
	switch in.V.Slots[in.V.Names[var_name]].Type {
	case INT:
		return in.V.Ints[ind]
	case FLOAT:
		return in.V.Floats[ind]
	case STR:
		return in.V.Strs[ind]
	case LIST:
		return in.V.Lists[ind]
	case BOOL:
		return in.V.Bools[ind]
	case BYTE:
		return in.V.Bytes[ind]
	case ARR:
		return in.V.Arrs[ind]
	case FUNC:
		return in.V.Funcs[ind]
	case SPAN:
		return in.V.Spans[ind]
	case ID:
		return in.V.Ids[ind]
	case PAIR:
		return in.V.Pairs[ind]
	}
	return nil
}

func (in *Interpreter) GetAnyRef(ref uint64) any {
	ind := in.V.Slots[ref].Index
	switch in.V.Slots[ref].Type {
	case INT:
		return in.V.Ints[ind]
	case FLOAT:
		return in.V.Floats[ind]
	case STR:
		return in.V.Strs[ind]
	case LIST:
		return in.V.Lists[ind]
	case BOOL:
		return in.V.Bools[ind]
	case BYTE:
		return in.V.Bytes[ind]
	case ARR:
		return in.V.Arrs[ind]
	case FUNC:
		return in.V.Funcs[ind]
	case PAIR:
		return in.V.Pairs[ind]
	}
	return nil
}

func PairKey(in *Interpreter, iname any) string {
	key_ref := in.GetRef(iname)
	// TODO: update dtype
	dtype := map[byte]string{0: "noth", 1: "int", 2: "float", 3: "str", 4: "arr", 5: "list", 6: "pair", 7: "bool", 8: "byte", 9: "func", 10: "id"}
	var iname_str string
	switch in.V.Slots[key_ref].Type { //TODO: add all types
	case INT:
		iname_str = in.V.Ints[in.V.Slots[key_ref].Index].String()
	case FLOAT:
		iname_str = in.V.Floats[in.V.Slots[key_ref].Index].String()
	case BYTE:
		iname_str = fmt.Sprintf("b.%d", in.V.Bytes[in.V.Slots[key_ref].Index])
	case BOOL:
		iname_str = ternary(in.V.Bools[in.V.Slots[key_ref].Index], "true", "false")
	case FUNC:
		iname_str = fmt.Sprintf("func.%s", in.V.Funcs[in.V.Slots[key_ref].Index].Name)
	case LIST:
		l := in.V.Lists[in.V.Slots[key_ref].Index]
		iname_str = ListString(&l, in)
	case PAIR:
		p := in.V.Pairs[in.V.Slots[key_ref].Index]
		iname_str = PairString(&p, in)
	case STR:
		iname_str = in.V.Strs[in.V.Slots[key_ref].Index]
	}
	key := fmt.Sprintf("%s:%s", dtype[in.V.Slots[key_ref].Type], iname_str)
	return key
}

func NewInterpreter(code, file string) Interpreter {
	in := Interpreter{}
	in.Code = bytecode.GetCode(code)
	in.File = &file
	in.V = &Vars{}
	in.V.Names = make(map[string]int)
	for _, fn := range bytecode.GenerateFuns() {
		in.Save(fn.Name, &fn)
	}
	in.V.gcMax = 100
	return in
}

func (in *Interpreter) EqualizeTypes(v1, v2 string) (string, string) { // returns tempvar names
	t1, t2 := in.V.Slots[in.V.Names[v1]].Type, in.V.Slots[in.V.Names[v2]].Type
	if t1 == t2 {
		return v1, v2
	}
	if t1 == INT && t2 == FLOAT {
		converted := big.NewFloat(0)
		converted.SetString(in.V.Ints[in.V.Slots[in.V.Names[v1]].Index].String())
		in.Save("_temp_a", converted)
		return "_temp_a", v2
	} else if t1 == FLOAT && t2 == INT {
		converted := big.NewFloat(0)
		converted.SetString(in.V.Ints[in.V.Slots[in.V.Names[v2]].Index].String())
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
		converted := big.NewInt(int64(in.V.Bytes[in.V.Slots[in.V.Names[v1]].Index]))
		in.Save("_temp_a", converted)
		return "_temp_a", v2
	} else if t1 == INT && t2 == BYTE {
		converted := big.NewInt(int64(in.V.Bytes[in.V.Slots[in.V.Names[v2]].Index]))
		in.Save("_temp_a", converted)
		return v1, "_temp_a"
	} else if t1 == BYTE && t2 == FLOAT {
		converted := big.NewFloat(float64(in.V.Bytes[in.V.Slots[in.V.Names[v1]].Index]))
		in.Save("_temp_a", converted)
		return "_temp_a", v2
	} else if t1 == FLOAT && t2 == BYTE {
		converted := big.NewFloat(float64(in.V.Bytes[in.V.Slots[in.V.Names[v2]].Index]))
		in.Save("_temp_a", converted)
		return v1, "_temp_a"
	}
	return v1, v2
}

func (in *Interpreter) GC() {
	in.V.gcCycle++
	if in.V.gcCycle >= in.V.gcMax {
		in.V.gcCycle = 0
	} else {
		return
	}
	old := in.V
	newVars := &Vars{
		Names: make(map[string]int),
	}

	// Maps old slot indices to new ones per type
	intMap := map[int]int{}
	floatMap := map[int]int{}
	strMap := map[int]int{}
	boolMap := map[int]int{}
	byteMap := map[int]int{}
	idMap := map[int]int{}
	funcMap := map[int]int{}
	arrMap := map[int]int{}
	spanMap := map[int]int{}
	listMap := map[int]int{}
	pairMap := map[int]int{}
	slotMap := map[int]int{} // exp

	var copyEntry func(e Entry) int
	copyEntry = func(e Entry) int {
		switch e.Type {
		case INT:
			if idx, ok := intMap[e.Index]; ok {
				return idx
			}
			newIndex := len(newVars.Ints)
			newVars.Ints = append(newVars.Ints, old.Ints[e.Index])
			intMap[e.Index] = newIndex
			return newIndex
		case FLOAT:
			if idx, ok := floatMap[e.Index]; ok {
				return idx
			}
			newIndex := len(newVars.Floats)
			newVars.Floats = append(newVars.Floats, old.Floats[e.Index])
			floatMap[e.Index] = newIndex
			return newIndex
		case STR:
			if idx, ok := strMap[e.Index]; ok {
				return idx
			}
			newIndex := len(newVars.Strs)
			newVars.Strs = append(newVars.Strs, old.Strs[e.Index])
			strMap[e.Index] = newIndex
			return newIndex
		case BOOL:
			if idx, ok := boolMap[e.Index]; ok {
				return idx
			}
			newIndex := len(newVars.Bools)
			newVars.Bools = append(newVars.Bools, old.Bools[e.Index])
			boolMap[e.Index] = newIndex
			return newIndex
		case BYTE:
			if idx, ok := byteMap[e.Index]; ok {
				return idx
			}
			newIndex := len(newVars.Bytes)
			newVars.Bytes = append(newVars.Bytes, old.Bytes[e.Index])
			byteMap[e.Index] = newIndex
			return newIndex
		case ID:
			if idx, ok := idMap[e.Index]; ok {
				return idx
			}
			newIndex := len(newVars.Ids)
			newVars.Ids = append(newVars.Ids, old.Ids[e.Index])
			idMap[e.Index] = newIndex
			return newIndex
		case FUNC:
			if idx, ok := funcMap[e.Index]; ok {
				return idx
			}
			newIndex := len(newVars.Funcs)
			newVars.Funcs = append(newVars.Funcs, old.Funcs[e.Index])
			funcMap[e.Index] = newIndex
			return newIndex
		case ARR:
			if idx, ok := arrMap[e.Index]; ok {
				return idx
			}
			val := old.Arrs[e.Index]
			// Assume Arrays hold only primitive types
			newIndex := len(newVars.Arrs)
			newVars.Arrs = append(newVars.Arrs, val)
			arrMap[e.Index] = newIndex
			return newIndex
		case SPAN:
			if idx, ok := spanMap[e.Index]; ok {
				return idx
			}
			val := old.Spans[e.Index]
			newSpan := bytecode.Span{Dtype: val.Dtype, Length: val.Length}
			switch val.Dtype {
			case INT:
				newSpan.Start = uint64(len(newVars.Ints))
				newVars.Ints = append(newVars.Ints, old.Ints[val.Start:val.Start+val.Length]...)
			}
			newIndex := len(newVars.Spans)
			newVars.Spans = append(newVars.Spans, newSpan)
			spanMap[e.Index] = newIndex
			return newIndex
		case LIST:
			if idx, ok := listMap[e.Index]; ok {
				return idx
			}
			val := old.Lists[e.Index]
			var newList bytecode.List
			for _, slot := range val.Ids {
				oldEntry := old.Slots[slot]
				newSlot := copyEntry(oldEntry)
				newList.Ids = append(newList.Ids, uint64(len(newVars.Slots)))
				newVars.Slots = append(newVars.Slots, Entry{Type: oldEntry.Type, Index: newSlot})
			}
			newIndex := len(newVars.Lists)
			newVars.Lists = append(newVars.Lists, newList)
			listMap[e.Index] = newIndex
			return newIndex
		case PAIR:
			if idx, ok := pairMap[e.Index]; ok {
				return idx
			}
			val := old.Pairs[e.Index]
			var newPair bytecode.Pair
			newPair.Ids = make(map[string]uint64)
			for key, slot := range val.Ids {
				oldEntry := old.Slots[slot]
				newSlot := copyEntry(oldEntry)
				// old version:
				//newPair.Ids = append(newPair.Ids, uint64(len(newVars.Slots)))
				newPair.Ids[key] = uint64(len(newVars.Slots))
				newVars.Slots = append(newVars.Slots, Entry{Type: oldEntry.Type, Index: newSlot})
			}
			newIndex := len(newVars.Pairs)
			newVars.Pairs = append(newVars.Pairs, newPair)
			pairMap[e.Index] = newIndex
			return newIndex
		default:
			return -1
		}
	}

	// Start GC traversal from named variables
	for name, slotIdx := range old.Names {
		if strings.HasPrefix(name, "_temp_") {
			continue
		}
		oldEntry := old.Slots[slotIdx]
		newIndex := copyEntry(oldEntry)
		newSlotIndex := len(newVars.Slots) // exp
		newVars.Names[name] = newSlotIndex
		newVars.Slots = append(newVars.Slots, Entry{Type: oldEntry.Type, Index: newIndex})
		slotMap[slotIdx] = newSlotIndex

	}

	// exp
	// Update all entries in newVars.Ids to reflect updated slot mappings
	for i, oldSlot := range newVars.Ids {
		if newSlot, ok := slotMap[int(oldSlot)]; ok {
			newVars.Ids[i] = uint64(newSlot)
		} else {
			// Clear invalid or collected references
			newVars.Ids[i] = 0
		}
	}

	newVars.gcCycle = old.gcCycle
	newVars.gcMax = old.gcMax
	in.V = newVars
	runtime.GC()
}

func (in *Interpreter) Compile(code, fname string) {
	in2 := Interpreter{V: &Vars{
		Names: make(map[string]int),
	}}
	in2.Code = bytecode.GetCode(code)
	in2.File = &fname
	for key := range in2.Code {
		in.Code[key] = in2.Code[key]
		delete(in2.Code, key)
	}
}

func Test() {
	code := `func sqrt x:
    return x^0.5
!print !sqrt 9
if 1==2:
    !print "eq"
else:
    !print "uneq"`
	in := NewInterpreter(code, "none")
	in.Run(fmt.Sprintf("_node_%d", bytecode.NodeN-1))
}

func StartFull(code, file string) {
	in := NewInterpreter(code, file)
	in.Run(fmt.Sprintf("_node_%d", bytecode.NodeN-1))
}

var error_type, error_message string

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
		// println(message)
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

func (in *Interpreter) GetSlot(name string) Entry {
	return in.V.Slots[in.V.Names[name]]
}

func (in *Interpreter) CheckDtype(action bytecode.Action, index int, dtypes ...byte) bool {
	dstrings := map[byte]string{0: "noth", 1: "int", 2: "float", 3: "str", 4: "arr", 5: "list", 6: "pair", 7: "bool", 8: "byte", 9: "func", 10: "id"}
	found := false
	for _, dtype := range dtypes {
		// in.V.Slots[in.V.Names[string(action.Variables[index])]].Type
		if in.V.Slots[in.V.Names[string(action.Variables[index])]].Type == dtype {
			found = true
			break
		}
	}
	if !found {
		l := bytecode.List{}
		for _, dtype := range dtypes {
			ListAppend(&l, in, dstrings[dtype])
		}
		in.Error(action, fmt.Sprintf("argument %d (%s) is %s, must be one of: %s!", index, string(action.Variables[index]), dstrings[in.GetSlot(string(action.Variables[index])).Type], ListString(&l, in)), "arg_type")
		return true
	}
	return false
}

func (in *Interpreter) GetRef(v any) uint64 {
	entry := Entry{TypeToByte(v), 0}
	switch val := v.(type) {
	case *big.Int:
		entry.Index = len(in.V.Ints)
		in.V.Ints = append(in.V.Ints, val)
	case *big.Float:
		entry.Index = len(in.V.Floats)
		in.V.Floats = append(in.V.Floats, val)
	case string:
		entry.Index = len(in.V.Strs)
		in.V.Strs = append(in.V.Strs, val)
	case byte:
		entry.Index = len(in.V.Bytes)
		in.V.Bytes = append(in.V.Bytes, val)
	case bool:
		entry.Index = len(in.V.Bools)
		in.V.Bools = append(in.V.Bools, val)
	case uint64:
		entry.Index = len(in.V.Ids)
		in.V.Ids = append(in.V.Ids, val)
	case bytecode.List:
		entry.Index = len(in.V.Lists)
		in.V.Lists = append(in.V.Lists, val)
	case bytecode.Array:
		entry.Index = len(in.V.Arrs)
		in.V.Arrs = append(in.V.Arrs, val)
	case bytecode.Pair:
		entry.Index = len(in.V.Pairs)
		in.V.Pairs = append(in.V.Pairs, val)
	case *bytecode.Function:
		entry.Index = len(in.V.Funcs)
		in.V.Funcs = append(in.V.Funcs, val)
	case bytecode.Span:
		entry.Index = len(in.V.Spans)
		in.V.Spans = append(in.V.Spans, val)
	}
	value := len(in.V.Slots)
	in.V.Slots = append(in.V.Slots, entry)
	return uint64(value)
}

func ListAppend(l *bytecode.List, in *Interpreter, item any) {
	ref := in.GetRef(item)
	l.Ids = append(l.Ids, ref)
}

func ListString(l *bytecode.List, in *Interpreter) string {
	elements := []string{}
	for _, item_id := range l.Ids {
		item := in.V.Slots[item_id].Index
		switch in.V.Slots[item_id].Type {
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
		switch in.V.Slots[item].Type {
		case INT:
			elements = append(elements, dkey+": "+in.V.Ints[in.V.Slots[item].Index].String())
		case FLOAT:
			elements = append(elements, dkey+": "+in.V.Floats[in.V.Slots[item].Index].String())
		case BOOL:
			elements = append(elements, dkey+": "+ternary(in.V.Bools[in.V.Slots[item].Index], "true", "false"))
		case BYTE:
			elements = append(elements, dkey+": "+fmt.Sprintf("b.%d", in.V.Bytes[in.V.Slots[item].Index]))
		case STR:
			elements = append(elements, dkey+": \""+in.V.Strs[in.V.Slots[item].Index]+"\"")
		case LIST:
			ll := in.V.Lists[in.V.Slots[item].Index]
			elements = append(elements, dkey+": "+ListString(&ll, in))
		case FUNC:
			elements = append(elements, dkey+": "+"func."+in.V.Funcs[in.V.Slots[item].Index].Name)
		case PAIR:
			pp := in.V.Pairs[in.V.Slots[item].Index]
			elements = append(elements, dkey+": "+PairString(&pp, in))
		}
	}
	return "{" + strings.Join(elements, ", ") + "}"
}

// GETTING VARIABLES START
func (in *Interpreter) NamedInt(vname string) *big.Int {
	return in.V.Ints[in.V.Slots[in.V.Names[vname]].Index]
}
func (in *Interpreter) NamedFloat(vname string) *big.Float {
	return in.V.Floats[in.V.Slots[in.V.Names[vname]].Index]
}
func (in *Interpreter) NamedStr(vname string) string {
	return in.V.Strs[in.V.Slots[in.V.Names[vname]].Index]
}
func (in *Interpreter) NamedBool(vname string) bool {
	return in.V.Bools[in.V.Slots[in.V.Names[vname]].Index]
}
func (in *Interpreter) NamedByte(vname string) byte {
	return in.V.Bytes[in.V.Slots[in.V.Names[vname]].Index]
}
func (in *Interpreter) NamedFunc(vname string) *bytecode.Function {
	return in.V.Funcs[in.V.Slots[in.V.Names[vname]].Index]
}
func (in *Interpreter) NamedList(vname string) bytecode.List {
	return in.V.Lists[in.V.Slots[in.V.Names[vname]].Index]
}
func (in *Interpreter) NamedArr(vname string) bytecode.Array {
	return in.V.Arrs[in.V.Slots[in.V.Names[vname]].Index]
}
func (in *Interpreter) NamedSpan(vname string) bytecode.Span {
	return in.V.Spans[in.V.Slots[in.V.Names[vname]].Index]
}
func (in *Interpreter) NamedPair(vname string) bytecode.Pair {
	return in.V.Pairs[in.V.Slots[in.V.Names[vname]].Index]
}
func (in *Interpreter) NamedId(vname string) uint64 {
	return in.V.Ids[in.V.Slots[in.V.Names[vname]].Index]
}

// GETTING VARIABLES END

func PairAppend(p *bytecode.Pair, in *Interpreter, item any, iname any) {
	value_ref := in.GetRef(item)
	key := PairKey(in, iname)
	p.Ids[key] = value_ref
}

func (in *Interpreter) RemoveName(name string) {
	_, ok := in.V.Names[name]
	if ok {
		delete(in.V.Names, name)
	}
}

func PowInt(number, power *big.Int) *big.Int {
	result := big.NewInt(0)
	result.Set(power.Exp(number, power, nil))
	return result
}

func (in *Interpreter) NewSpan(length int, dtype byte) bytecode.Span {
	if length == 0 {
		return bytecode.Span{}
	} else {
		s := bytecode.Span{}
		s.Dtype = dtype
		switch dtype {
		case INT:
			s.Start = uint64(len(in.V.Ints))
			s.Length = uint64(length)
			for range length {
				in.V.Ints = append(in.V.Ints, big.NewInt(0))
			}
		default:
			panic("unsupported Span content")
		}
		return s
	}
}

func (in *Interpreter) StringSpan(l bytecode.Span) string {
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

func (in *Interpreter) Parse(str string) []string {
	reg_var := regexp.MustCompile(`\{.+?\}`)
	for _, match := range reg_var.FindAllString(str, -1) {
		code := match[1 : len(match)-1]
		if variable, ok := in.V.Names[code]; ok {
			a := in.GetAnyRef(uint64(variable))
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
	return GetParts(str)
}

func GetParts(text string) []string {
	mode := 0 // 0 = normal, 1 = single-quote, 2 = double-quote
	runes := []rune(text)
	parts := []string{""}
	escape := false

	for _, r := range runes {
		if escape {
			// Always append the escaped char literally
			parts[len(parts)-1] += string(r)
			escape = false
			continue
		}

		if r == '\\' {
			// Start escape sequence
			escape = true
			continue
		}

		switch mode {
		case 2: // inside double quotes
			if r == '"' {
				mode = 0
			} else {
				parts[len(parts)-1] += string(r)
			}
		case 1: // inside single quotes
			if r == '\'' {
				mode = 0
			} else {
				parts[len(parts)-1] += string(r)
			}
		case 0: // normal
			if r == ' ' || r == '\t' {
				if parts[len(parts)-1] != "" {
					parts = append(parts, "")
				}
			} else if r == '"' {
				mode = 2
			} else if r == '\'' {
				mode = 1
			} else {
				parts[len(parts)-1] += string(r)
			}
		}
	}

	// If the string ended with a backslash, keep it
	if escape {
		parts[len(parts)-1] += "\\"
	}

	// Trim trailing empty argument if present
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	return parts
}

func (in *Interpreter) Fmt(str string) string {
	reg_var := regexp.MustCompile(`\{.+?\}`)
	for _, match := range reg_var.FindAllString(str, -1) {
		code := match[1 : len(match)-1]
		if variable, ok := in.V.Names[code]; ok {
			a := in.GetAnyRef(uint64(variable))
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
	return str
}

var RL *readline.Instance
var protected_actions = []string{"for", "const", "pool"}

// MAIN FUNCTION START
func (in *Interpreter) Run(node_name string) bool {
	actions := in.Code[node_name]
	focus := 0
	for focus < len(actions) && !in.halt {
		action := actions[focus]
		for _, vv := range action.Variables {
			if _, ok := in.V.Names[string(vv)]; !ok && !bytecode.Has(protected_actions, action.Type) {
				in.Error(action, "Undeclared variable: "+string(vv), "undeclared")
				return true
			}
		}
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
		case "+":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Slots[in.V.Names[o]].Type {
			case INT:
				in.Save(actions[focus].Target, big.NewInt(0))
				in.V.Ints[in.V.Slots[in.V.Names[actions[focus].Target]].Index].Add(in.V.Ints[in.V.Slots[in.V.Names[o]].Index], in.V.Ints[in.V.Slots[in.V.Names[t]].Index])
			case FLOAT:
				in.Save(actions[focus].Target, big.NewFloat(0))
				in.V.Floats[in.V.Slots[in.V.Names[actions[focus].Target]].Index].Add(in.V.Floats[in.V.Slots[in.V.Names[o]].Index], in.V.Floats[in.V.Slots[in.V.Names[t]].Index])
			case STR:
				err := in.CheckDtype(action, 1, STR)
				if err {
					return true
				}
				in.Save(actions[focus].Target, in.NamedStr(o)+in.NamedStr(t))
			case BYTE:
				in.Save(actions[focus].Target, in.NamedByte(o)+in.NamedByte(t))
			}
		case "'", "''":
			switch in.V.Slots[in.V.Names[string(action.Variables[0])]].Type {
			case PAIR:
				p := in.NamedPair(string(action.Variables[0]))
				ind := PairKey(in, in.GetAny(string(action.Variables[1])))
				if action.Type == "'" {
					in.Save(action.Target, in.GetAnyRef(p.Ids[ind]))
				} else {
					in.V.Names[actions[focus].Target] = int(p.Ids[ind])
				}
			case LIST:
				// TODO: add errors and slice support
				l := in.NamedList(string(action.Variables[0]))
				ind := in.NamedInt(string(action.Variables[1])).Int64()
				if ind < 0 {
					ind += int64(len(l.Ids))
				}
				if action.Type == "'" {
					in.Save(action.Target, in.GetAnyRef(l.Ids[ind]))
				} else {
					in.V.Names[action.Target] = int(l.Ids[ind])
				}
			}
		case "deep":
			in.Save(string(action.Variables[0]), in.GetAny(string(action.Variables[1])))
		case "pair":
			p := bytecode.Pair{}
			p.Ids = make(map[string]uint64)
			for n := 0; n < len(actions[focus].Variables); n += 2 {
				PairAppend(&p, in, in.GetAny(string(actions[focus].Variables[n+1])), in.GetAny(string(actions[focus].Variables[n])))
			}
			in.Save(actions[focus].Target, p)
		case "-":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Slots[in.V.Names[o]].Type {
			case INT:
				in.Save(actions[focus].Target, big.NewInt(0))
				in.V.Ints[in.V.Slots[in.V.Names[actions[focus].Target]].Index].Sub(in.V.Ints[in.V.Slots[in.V.Names[o]].Index], in.V.Ints[in.V.Slots[in.V.Names[t]].Index])
			case FLOAT:
				in.Save(actions[focus].Target, big.NewFloat(0))
				in.V.Floats[in.V.Slots[in.V.Names[actions[focus].Target]].Index].Sub(in.V.Floats[in.V.Slots[in.V.Names[o]].Index], in.V.Floats[in.V.Slots[in.V.Names[t]].Index])
			case BYTE:
				in.Save(actions[focus].Target, in.NamedByte(o)-in.NamedByte(t))
			}
		case "*":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Slots[in.V.Names[o]].Type {
			case INT:
				in.Save(actions[focus].Target, big.NewInt(0))
				in.V.Ints[in.V.Slots[in.V.Names[actions[focus].Target]].Index].Mul(in.V.Ints[in.V.Slots[in.V.Names[o]].Index], in.V.Ints[in.V.Slots[in.V.Names[t]].Index])
			case FLOAT:
				in.Save(actions[focus].Target, big.NewFloat(0))
				in.V.Floats[in.V.Slots[in.V.Names[actions[focus].Target]].Index].Mul(in.V.Floats[in.V.Slots[in.V.Names[o]].Index], in.V.Floats[in.V.Slots[in.V.Names[t]].Index])
			case BYTE:
				in.Save(actions[focus].Target, in.NamedByte(o)*in.NamedByte(t))
			}
		case "/":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Slots[in.V.Names[o]].Type {
			case INT:
				f := big.NewFloat(0)
				f.SetInt(in.NamedInt(o))
				f2 := big.NewFloat(0)
				f2.SetInt(in.NamedInt(t))
				in.Save(action.Target, f.Quo(f, f2))
			case FLOAT:
				in.Save(actions[focus].Target, big.NewFloat(0))
				in.V.Floats[in.V.Slots[in.V.Names[actions[focus].Target]].Index].Quo(in.V.Floats[in.V.Slots[in.V.Names[o]].Index], in.V.Floats[in.V.Slots[in.V.Names[t]].Index])
			case BYTE:
				// TODO: make it work according to the spec
				in.Save(actions[focus].Target, in.NamedByte(o)/in.NamedByte(t))
			}
		case "//":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Slots[in.V.Names[o]].Type {
			case INT:
				f := big.NewFloat(0)
				f.SetInt(in.NamedInt(o))
				f2 := big.NewFloat(0)
				f2.SetInt(in.NamedInt(t))
				in.Save(action.Target, big.NewInt(0))
				in.NamedInt(action.Target).Div(in.NamedInt(o), in.NamedInt(t))
			case FLOAT:
				f := in.NamedFloat(o)
				f2 := in.NamedFloat(t)
				result := f.Quo(f, f2)
				rounded, _ := result.Int(big.NewInt(0))
				in.Save(action.Target, rounded)
			case BYTE:
				// TODO: make it work according to the spec
				in.Save(actions[focus].Target, in.NamedByte(o)/in.NamedByte(t))
			}
		case "%":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Slots[in.V.Names[o]].Type {
			case INT:
				in.Save(actions[focus].Target, big.NewInt(0))
				in.V.Ints[in.V.Slots[in.V.Names[actions[focus].Target]].Index].DivMod(in.V.Ints[in.V.Slots[in.V.Names[o]].Index], in.V.Ints[in.V.Slots[in.V.Names[t]].Index], in.V.Ints[in.V.Slots[in.V.Names[actions[focus].Target]].Index])
			case FLOAT:
				// TODO: make it work according to the spec
				in.Save(actions[focus].Target, big.NewFloat(0))
				in.V.Floats[in.V.Slots[in.V.Names[actions[focus].Target]].Index].Sub(in.V.Floats[in.V.Slots[in.V.Names[o]].Index], in.V.Floats[in.V.Slots[in.V.Names[t]].Index])
			case BYTE:
				in.Save(actions[focus].Target, in.NamedByte(o)-in.NamedByte(t))
			}
		case "^":
			one, two := in.EqualizeTypes(string(action.Variables[0]), string(action.Variables[1]))
			switch in.V.Slots[in.V.Names[one]].Type {
			case INT:
				in.Save(action.Target, in.NamedInt(one))
				in.Save(action.Target, PowInt(in.NamedInt(action.Target), in.NamedInt(two)))
			case FLOAT:
				o, _ := in.NamedFloat(one).Float64()
				t, _ := in.NamedFloat(two).Float64()
				in.Save(action.Target, big.NewFloat(math.Pow(o, t)))
			case BYTE:
				// TODO:
				//in.V.Bytes[in.V.Names[action.Target]] = byte(math.Pow(float64(in.V.Bytes[in.V.Names[string(action.Variables[0])]]), float64(in.V.Bytes[in.V.Names[string(action.Variables[1])]])))
				//in.V.Types[in.V.Names[action.Target]] = BYTE
			}
		case "func":
			name := string(actions[focus].Variables[0])
			fn := &bytecode.Function{name, actions[focus].Target, actions[focus].Variables[1:], actions[focus].Target}
			in.Save(name, fn)
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
		case "pool_trash":
			// --- parse variables ---
			input_lefts := []string{}
			input_rights := []string{}
			output_lefts := []string{}
			output_rights := []string{}

			changed := false
			vars := action.Variables
			i := 0
			for i < len(vars) {
				left := string(vars[i])
				if left == "Nothing" {
					changed = true
					i++
					continue
				}
				right := ""
				if i+1 < len(vars) {
					right = string(vars[i+1])
				}
				if !changed {
					input_lefts = append(input_lefts, left)
					input_rights = append(input_rights, right)
				} else {
					output_lefts = append(output_lefts, left)
					output_rights = append(output_rights, right)
				}
				i += 2
			}

			// must have at least one input
			//if len(input_lefts) == 0 {
			//	return in.Error(action, "pool requires at least one input list", "arg_count")
			//}

			cores := runtime.NumCPU()
			if cores < 1 {
				cores = 1
			}

			// --- partition each input list into per-worker chunks ---
			// chunksPerInput[inputIndex][workerIndex] -> []uint64 (ids)
			chunksPerInput := make([][][]uint64, len(input_lefts))
			for idx, key := range input_lefts {
				src := in.NamedList(key)
				total := len(src.Ids)
				chunksPerInput[idx] = make([][]uint64, cores)
				for w := 0; w < cores; w++ {
					start := w * total / cores
					end := (w + 1) * total / cores
					if start < end {
						chunksPerInput[idx][w] = append([]uint64(nil), src.Ids[start:end]...)
					} else {
						chunksPerInput[idx][w] = []uint64{}
					}
				}
			}

			// --- create worker interpreters and save their chunk lists locally ---
			interpreters := make([]*Interpreter, cores)
			workerChunkNames := make([][]string, cores) // per-worker list of names saved in worker
			for w := 0; w < cores; w++ {
				f_in := &Interpreter{V: &Vars{Names: make(map[string]int)}}
				f_in.Copy2(in) // your copy routine that deep-copies needed data

				names := make([]string, len(input_lefts))
				for j, key := range input_lefts {
					chunkIds := chunksPerInput[j][w]
					var l bytecode.List
					for _, id := range chunkIds {
						// copy element into worker's Vars and append
						val := in.GetAnyRef(id)
						ListAppend(&l, f_in, val)
					}
					tmpName := fmt.Sprintf("_pool_chunk_w%d_input%d_%s", w, j, key)
					f_in.Save(tmpName, l)
					names[j] = tmpName
				}

				interpreters[w] = f_in
				workerChunkNames[w] = names
			}

			// --- start workers; each returns []bytecode.List (one list per output left) ---
			type workerResult struct {
				lists []bytecode.List // lists local to worker (IDs in worker's Vars)
				err   error
			}
			resultsChans := make([]chan workerResult, cores)
			for w := 0; w < cores; w++ {
				resultsChans[w] = make(chan workerResult, 1)
				go func(w int) {
					// worker closure
					win := interpreters[w]
					srcNames := workerChunkNames[w]
					// resolve chunks in worker
					chunks := make([]bytecode.List, len(srcNames))
					for j, nm := range srcNames {
						chunks[j] = win.NamedList(nm)
					}

					// prepare outputs (one list per output_lefts)
					outLists := make([]bytecode.List, len(output_lefts))
					for j := range outLists {
						outLists[j] = bytecode.List{}
					}

					// iterate indices (use shortest chunk length to be safe)
					length := 0
					if len(chunks) > 0 {
						length = len(chunks[0].Ids)
						for _, c := range chunks {
							if len(c.Ids) < length {
								length = len(c.Ids)
							}
						}
					}

					var werr error
					for idx := 0; idx < length; idx++ {
						// bind inputs inside worker
						for j := 0; j < len(chunks); j++ {
							id := chunks[j].Ids[idx]
							val := in.GetAnyRef(id)
							win.Save(input_rights[j], val)
						}

						// run the node body
						if err := win.Run(action.Target); err != false {
							// store last error, but continue other items
							werr = fmt.Errorf("worker error")
							continue
						}

						// collect outputs from worker variables, append into outLists
						for outIdx := range output_lefts {
							val := win.GetAny(output_rights[outIdx])
							ListAppend(&outLists[outIdx], win, val)
						}
					}

					resultsChans[w] <- workerResult{lists: outLists, err: werr}
				}(w)
			}

			// --- collect worker results and merge into main interpreter ---
			workerResults := make([]workerResult, cores)
			for w := 0; w < cores; w++ {
				workerResults[w] = <-resultsChans[w]
				close(resultsChans[w])
			}

			// init combined lists in main interpreter
			combinedLists := make([]bytecode.List, len(output_lefts))
			for i := range combinedLists {
				combinedLists[i] = bytecode.List{}
			}

			// merge: for each worker, for each outList element, fetch value from that worker and append to main
			for w := 0; w < cores; w++ {
				res := workerResults[w]
				win := interpreters[w]
				for outIdx := range res.lists {
					// res.lists[outIdx] contains IDs local to win
					for _, localId := range res.lists[outIdx].Ids {
						val := win.GetAnyRef(localId)
						ListAppend(&combinedLists[outIdx], in, val)
					}
				}
			}

			// save combined outputs into main interpreter
			for outIdx, outName := range output_lefts {
				in.Save(outName, combinedLists[outIdx])
			}

			// cleanup worker interpreters
			for _, win := range interpreters {
				win.Destroy()
			}
		case "pool":
			input_lefts := []string{}
			input_rights := []string{}
			output_lefts := []string{}
			output_rights := []string{}

			changed := false
			vars := action.Variables
			i := 0
			for i < len(vars) {
				left := string(vars[i])
				if left == "Nothing" {
					changed = true
					i++
					continue
				}
				right := ""
				if i+1 < len(vars) {
					right = string(vars[i+1])
				}
				if !changed {
					if _, ok := in.V.Names[left]; !ok {
						in.Error(action, "undeclared variable in pool statement: "+left, "undeclared")
						return true
					}
					if in.V.Slots[in.V.Names[left]].Type != LIST {
						in.Error(action, "non-list input in pool statement: "+left, "arg_type")
						return true
					}
					input_lefts = append(input_lefts, left)
					input_rights = append(input_rights, right)
				} else {
					// after sentinel: left is the output name in main, right is the worker var name
					output_lefts = append(output_lefts, left)
					output_rights = append(output_rights, right)
				}
				i += 2
			}

			cores := runtime.NumCPU()
			if cores < 1 {
				cores = 1
			}

			var interpreters []*Interpreter
			length := len(in.NamedList(input_lefts[0]).Ids)
			focus := 0.0
			step := float64(length) / float64(cores)
			for range cores {
				f_in := &Interpreter{V: &Vars{Names: make(map[string]int)}}
				f_in.Copy2(in)
				for n, left := range input_lefts {
					chunks := in.NamedList(input_lefts[n]).Ids[int(math.Round(focus)):int(math.Round(focus+step))]
					focus += step
					l := bytecode.List{}
					for _, chunk := range chunks {
						ListAppend(&l, f_in, in.GetAnyRef(chunk))
					}
					f_in.Save(left, l)
				}
				interpreters = append(interpreters, f_in)
			}
			channels := make([]chan bool, cores)
			for w := range cores {
				channels[w] = make(chan bool)
				go func(win *Interpreter, target string, input_lefts, input_rights, output_lefts, output_rights *[]string, ch chan bool) {
					length := len(win.NamedList((*input_lefts)[0]).Ids)
					// reply := make([]bytecode.List, len(*output_rights))
					for _, left_name := range *output_lefts {
						win.Save(left_name, bytecode.List{})
					}
					// end
					var err bool
					for i := 0; i < length; i++ {
						for j, left := range *input_lefts {
							win.Save((*input_rights)[j], win.GetAnyRef(win.NamedList(left).Ids[i]))
						}
						err = win.Run(target)
						for jj, name := range *output_rights {
							// ListAppend(&reply[jj], win, win.GetAny(name))
							l := win.NamedList((*output_lefts)[jj])
							ListAppend(&l, win, win.GetAny(name))
							win.Save((*output_lefts)[jj], l)
							// end
						}
					}
					/*
						for o_r, left := range *output_lefts {
							win.Save(left, reply[o_r])
							println(ListString(&reply[o_r], win)) // error occurs even earlier
						}
					*/
					ch <- err
				}(interpreters[w], action.Target, &input_lefts, &input_rights, &output_lefts, &output_rights, channels[w])
			}
			for idx := 0; idx < cores; idx++ {
				<-channels[idx]
				close(channels[idx])
			}
			for n, oleft := range output_lefts {
				total := bytecode.List{}
				for w := range interpreters {
					for _, id := range interpreters[w].NamedList(oleft).Ids {
						v := interpreters[w].GetAnyRef(id)
						ListAppend(&total, in, v)
					}
				}
				in.Save(output_lefts[n], total)
			}
			for _, worker := range interpreters {
				worker.Destroy()
			}
		case "pool2":
			// Parse variables: pairs before "Nothing" are inputs (left->right),
			// pairs after "Nothing" are outputs (left<-right).
			input_lefts := []string{}
			input_rights := []string{}
			output_lefts := []string{}
			output_rights := []string{}

			changed := false
			vars := action.Variables
			i := 0
			for i < len(vars) {
				left := string(vars[i])
				if left == "Nothing" {
					changed = true
					i++
					continue
				}
				right := ""
				if i+1 < len(vars) {
					right = string(vars[i+1])
				}
				if !changed {
					input_lefts = append(input_lefts, left)
					input_rights = append(input_rights, right)
				} else {
					// after sentinel: left is the output name in main, right is the worker var name
					output_lefts = append(output_lefts, left)
					output_rights = append(output_rights, right)
				}
				i += 2
			}

			cores := runtime.NumCPU()
			if cores < 1 {
				cores = 1
			}

			// Prepare per-worker interpreters and chunk names (saved into worker.V)
			var interpreters []*Interpreter
			var workerChunkNames [][]string // per-worker list of names that point to saved chunks

			for w := 0; w < cores; w++ {
				// create worker interpreter copy
				f_in := &Interpreter{V: &Vars{Names: make(map[string]int)}}
				f_in.Copy2(in)

				chunkNames := []string{}

				// For each input list key, create a chunk for this worker and save it into f_in
				for _, key := range input_lefts {
					srcList := in.NamedList(key)
					total := len(srcList.Ids)
					start := w * total / cores
					end := (w + 1) * total / cores
					if start >= end || total == 0 {
						// empty chunk
						empty := bytecode.List{}
						tmpName := fmt.Sprintf("_pool_chunk_%d_%s", w, key)
						f_in.Save(tmpName, empty)
						chunkNames = append(chunkNames, tmpName)
						continue
					}

					chunk := bytecode.List{}
					for ind := start; ind < end; ind++ {
						// copy the element into worker's Vars (ListAppend allocates local slots)
						val := in.GetAnyRef(srcList.Ids[ind])
						ListAppend(&chunk, f_in, val)
					}

					tmpName := fmt.Sprintf("_pool_chunk_%d_%s", w, key)
					f_in.Save(tmpName, chunk) // crucial: save into worker so IDs are local & valid
					chunkNames = append(chunkNames, tmpName)
				}

				interpreters = append(interpreters, f_in)
				workerChunkNames = append(workerChunkNames, chunkNames)
			}

			// Launch workers
			channels := make([]chan []bytecode.List, cores)
			for idx := range channels {
				channels[idx] = make(chan []bytecode.List, 1)
			}

			for idx := 0; idx < cores; idx++ {
				println("Worker:", idx)
				go PoolWorkerNew(workerChunkNames[idx], action.Target, input_rights, output_lefts, output_rights, channels[idx], interpreters[idx])
			}

			// Collect results (blocking receive per worker)
			processed := make([][]bytecode.List, 0, cores)
			for idx := 0; idx < cores; idx++ {
				res := <-channels[idx]
				processed = append(processed, res)
				close(channels[idx])
			}

			// Merge worker-local outputs into combined lists in the main interpreter
			for m := range output_lefts {
				combined := bytecode.List{}
				for w := 0; w < cores; w++ {
					// processed[w][m] contains a list whose Ids are local to interpreters[w]
					for _, id := range processed[w][m].Ids {
						// fetch value from worker's interpreter and append into main interpreter's combined list
						val := interpreters[w].GetAnyRef(id)
						ListAppend(&combined, in, val)
					}
				}
				in.Save(output_lefts[m], combined)
			}

			// Clean up worker interpreters
			for _, interp := range interpreters {
				interp.Destroy()
			}
		case "++":
			slot := in.GetSlot(string(action.Variables[0]))
			switch slot.Type {
			case INT:
				i := big.NewInt(0)
				i.Set(in.V.Ints[slot.Index])
				in.Save(actions[focus].Target, i)
				in.V.Ints[in.V.Slots[in.V.Names[action.Target]].Index].Add(in.V.Ints[slot.Index], big.NewInt(1))
			}
		case "--":
			slot := in.GetSlot(string(action.Variables[0]))
			switch slot.Type {
			case INT:
				i := big.NewInt(0)
				i.Set(in.V.Ints[slot.Index])
				in.Save(actions[focus].Target, i)
				in.V.Ints[in.V.Slots[in.V.Names[action.Target]].Index].Sub(in.V.Ints[slot.Index], big.NewInt(1))
			}
		case "=":
			in.Save(action.Target, in.GetAny(string(actions[focus].Variables[0])))
			/*
				switch in.GetSlot(string(action.Variables[0])).Type {
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
			*/
		case "&=":
			in.V.Names[action.Target] = in.V.Names[string(actions[focus].Variables[0])]
			// id := in.V.Names[string(actions[focus].Variables[0])]
			// in.V.Names[actions[focus].Target] = id
			// u := in.V.Names[string(actions[focus].Variables[0])]
			// in.Save(actions[focus].Target, u)
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
		case "==":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Slots[in.V.Names[o]].Type {
			case INT:
				r := in.NamedInt(o).Cmp(in.NamedInt(t))
				in.Save(actions[focus].Target, r == 0)
			case FLOAT:
				r := in.NamedFloat(o).Cmp(in.NamedFloat(t))
				in.Save(actions[focus].Target, r == 0)
			case BYTE:
				in.Save(action.Target, in.NamedByte(o) == in.NamedByte(t))
			case STR:
				in.Save(action.Target, in.NamedStr(o) == in.NamedStr(t))
			}
		case "!=":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Slots[in.V.Names[o]].Type {
			case INT:
				r := in.NamedInt(o).Cmp(in.NamedInt(t))
				in.Save(actions[focus].Target, r != 0)
			case FLOAT:
				r := in.NamedFloat(o).Cmp(in.NamedFloat(t))
				in.Save(actions[focus].Target, r != 0)
			case BYTE:
				in.Save(action.Target, in.NamedByte(o) != in.NamedByte(t))
			case STR:
				in.Save(action.Target, in.NamedStr(o) != in.NamedStr(t))
			}
		case "<":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Slots[in.V.Names[o]].Type {
			case INT:
				r := in.NamedInt(o).Cmp(in.NamedInt(t))
				in.Save(actions[focus].Target, r == -1)
			case FLOAT:
				r := in.NamedFloat(o).Cmp(in.NamedFloat(t))
				in.Save(actions[focus].Target, r == -1)
			case BYTE:
				in.Save(action.Target, in.NamedByte(o) < in.NamedByte(t))
			}
		case ">":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			switch in.V.Slots[in.V.Names[o]].Type {
			case INT:
				r := in.NamedInt(o).Cmp(in.NamedInt(t))
				in.Save(actions[focus].Target, r == 1)
			case FLOAT:
				r := in.NamedFloat(o).Cmp(in.NamedFloat(t))
				in.Save(actions[focus].Target, r == 1)
			case BYTE:
				in.Save(action.Target, in.NamedByte(o) > in.NamedByte(t))
			}
		case "if":
			err := in.CheckDtype(actions[focus], 0, BOOL)
			if err {
				return true
			}
			b := in.NamedBool(string(actions[focus].Variables[0]))
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
			b := in.NamedBool(string(actions[focus].Variables[0]))
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
			in.Save("_case"+action.Target, in.GetAny(string(action.Variables[0])))
			err := in.Run(action.Target)
			if err {
				return true
			}
			in.RemoveName("_case" + action.Target)
		case "case":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), "_case"+node_name)
			switch in.V.Slots[in.V.Names[o]].Type {
			case INT:
				r := in.NamedInt(o).Cmp(in.NamedInt(t))
				if r == 0 {
					err := in.Run(action.Target)
					return err
				}
			case FLOAT:
				r := in.NamedFloat(o).Cmp(in.NamedFloat(t))
				if r == 0 {
					err := in.Run(action.Target)
					return err
				}
			case BYTE:
				if in.NamedByte(o) == in.NamedByte(t) {
					err := in.Run(action.Target)
					return err
				}
			case STR:
				if in.NamedStr(o) == in.NamedStr(t) {
					err := in.Run(action.Target)
					return err
				}
			}
		case "for":
			// Spans are copied in here for safety
			sources := []string{}
			targets := []string{}
			loopLen := uint64(0)

			for i := 0; i < len(action.Variables); i += 2 {
				targetName := string(action.Variables[i+1])
				spanName := string(action.Variables[i])

				if in.V.Slots[in.V.Names[spanName]].Type == SPAN {
					// Get and copy the span to protect it from mutation
					orig := in.NamedSpan(spanName)
					loopLen = orig.Length
					copied := bytecode.Span{
						Dtype:  orig.Dtype,
						Start:  orig.Start,
						Length: orig.Length,
					}

					cname := "_for" + action.Target + "_" + spanName
					sources = append(sources, cname)
					in.Save(cname, copied)
				} else if in.V.Slots[in.V.Names[spanName]].Type == STR {
					copied := in.NamedStr(spanName)
					loopLen = uint64(len([]rune(copied)))
					cname := "_for" + action.Target + "_" + spanName
					sources = append(sources, cname)
					in.Save(cname, copied)
				} else if in.V.Slots[in.V.Names[spanName]].Type == LIST {
					copied := in.NamedList(spanName)
					loopLen = uint64(len(copied.Ids))
					cname := "_for" + action.Target + "_" + spanName
					sources = append(sources, cname)
					in.Save(cname, copied)
				}
				targets = append(targets, targetName)
			}

			for idx := uint64(0); idx < loopLen; idx++ {
				for i, span_name := range sources {
					switch in.V.Slots[in.V.Names[span_name]].Type {
					case SPAN:
						span := in.NamedSpan(span_name)
						valIndex := span.Start + idx
						switch span.Dtype {
						case INT:
							in.Save(targets[i], in.V.Ints[valIndex])
						case FLOAT:
							in.Save(targets[i], in.V.Floats[valIndex])
						case STR:
							in.Save(targets[i], in.V.Strs[valIndex])
						case BOOL:
							in.Save(targets[i], in.V.Bools[valIndex])
						case BYTE:
							in.Save(targets[i], in.V.Bytes[valIndex])
						case ID:
							in.Save(targets[i], in.V.Ids[valIndex])
						default:
							panic("unsupported span dtype in for loop")
						}
					case STR:
						str := in.NamedStr(span_name)
						runes := []rune(str)
						in.Save(targets[i], string(runes[idx]))
					case LIST:
						l := in.NamedList(span_name)
						valIndex := l.Ids[idx]
						switch in.V.Slots[valIndex].Type {
						case INT:
							in.Save(targets[i], in.V.Ints[in.V.Slots[valIndex].Index])
						case FLOAT:
							in.Save(targets[i], in.V.Floats[in.V.Slots[valIndex].Index])
						case STR:
							in.Save(targets[i], in.V.Strs[in.V.Slots[valIndex].Index])
						case BOOL:
							in.Save(targets[i], in.V.Bools[in.V.Slots[valIndex].Index])
						case BYTE:
							in.Save(targets[i], in.V.Bytes[in.V.Slots[valIndex].Index])
						case ID:
							in.Save(targets[i], in.V.Ids[in.V.Slots[valIndex].Index])
						default:
							panic("unsupported span dtype in for loop")
						}
					}
				}

				// Here is where the execution actually happens
				if err := in.Run(action.Target); err {
					return true
				}
			}

			// Span copy cleanup
			for _, span_name := range sources {
				in.RemoveName(span_name)
			}
		case "$":
			text_command := strings.TrimSpace(strings.SplitN(action.Source.Source, "$", 2)[1])
			arguments := in.Parse(text_command)
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
			arguments := in.Parse(text_command)
			cmd := exec.Command(arguments[0], arguments[1:]...)
			out, go_err := cmd.CombinedOutput()
			if go_err != nil {
				in.Error(action, fmt.Sprintf("Error executing command: %v", go_err), "sys")
				return true
			} else {
				in.Save(action.Target, string(out))
			}
		case "GC":
			in.GC()
		default:
			fn := in.GetAny(action.Type).(*bytecode.Function) //in.V.Funcs[in.V.Names[actions[focus].Type]]
			// TODO: add boundcheck
			/*
				if !ok {
					in.Error(actions[focus], "Undeclared function!", "undeclared")
				}
			*/
			switch fn.Name {
			case "print":
				for _, v := range action.Variables {
					slot := in.GetSlot(string(v))
					switch slot.Type {
					case INT:
						fmt.Printf("%s ", in.V.Ints[slot.Index].String())
					case FLOAT:
						fmt.Printf("%s ", in.V.Floats[slot.Index].String())
					case BYTE:
						fmt.Printf("b.%d ", in.NamedByte(string(v)))
					case BOOL:
						fmt.Printf("%v ", in.NamedBool(string(v)))
					case FUNC:
						fmt.Printf("func.%s ", in.NamedFunc(string(v)).Name)
					case STR:
						fmt.Printf("%s ", in.NamedStr(string(v)))
					case SPAN:
						fmt.Printf("%s ", in.StringSpan(in.NamedSpan(string(v))))
					case ID:
						fmt.Printf("id.%d ", in.NamedId(string(v)))
					case LIST:
						l := in.NamedList(string(v))
						fmt.Print(ListString(&l, in) + " ")
					case PAIR:
						p := in.NamedPair(string(v))
						fmt.Print(PairString(&p, in) + " ")
					}
				}
				fmt.Println()
			case "fmt":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return true
				}
				str := in.Fmt(in.NamedStr(string(action.Variables[0])))
				in.Save(action.Target, str)
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
					os.Setenv(in.NamedStr(string(action.Variables[0])), in.NamedStr(string(action.Variables[1])))
				} else {
					err = in.CheckDtype(action, 0, STR)
					if err {
						return err
					}
					in.Save(action.Target, os.Getenv(in.NamedStr(string(action.Variables[0]))))
				}
			case "convert":
				err := in.CheckArgN(action, 2, 2)
				if err {
					return true
				}
				target_type := in.V.Slots[in.V.Names[string(action.Variables[1])]].Type
				if target_type == in.V.Slots[in.V.Names[string(action.Variables[0])]].Type {
					in.Save(action.Target, in.GetAny(string(action.Variables[0])))
				} else {
					switch target_type {
					case STR:
						switch in.V.Slots[in.V.Names[string(action.Variables[0])]].Type {
						case INT:
							in.Save(action.Target, in.NamedInt(string(action.Variables[0])).String())
						case FLOAT:
							in.Save(action.Target, in.NamedFloat(string(action.Variables[0])).String())
						case BYTE:
							in.Save(action.Target, fmt.Sprintf("b.%d", in.NamedByte(string(action.Variables[0]))))
						case BOOL:
							in.Save(action.Target, fmt.Sprintf("%v", in.NamedBool(string(action.Variables[0]))))
						}
					case INT:
						switch in.V.Slots[in.V.Names[string(action.Variables[0])]].Type {
						case FLOAT:
							v, _ := in.NamedFloat(string(action.Variables[0])).Int(big.NewInt(0))
							in.Save(action.Target, v)
						case BYTE:
							v, _ := in.NamedFloat(string(action.Variables[0])).Int64()
							in.Save(action.Target, byte(v))
						}
					case FLOAT:
						switch in.V.Slots[in.V.Names[string(action.Variables[0])]].Type {
						case INT:
							v, _ := in.NamedInt(string(action.Variables[0])).Float64()
							in.Save(action.Target, big.NewFloat(v))
						case BYTE:
							in.Save(action.Target, big.NewFloat(float64(in.NamedByte(string(action.Variables[0])))))
						}
					case BYTE:
						switch in.V.Slots[in.V.Names[string(action.Variables[0])]].Type {
						case FLOAT:
							v, _ := in.NamedFloat(string(action.Variables[0])).Int64()
							in.Save(action.Target, byte(v))
						case INT:
							v := in.NamedInt(string(action.Variables[0])).Int64()
							in.Save(action.Target, byte(v))
						}
					case LIST:
						switch in.V.Slots[in.V.Names[string(action.Variables[0])]].Type {
						case SPAN:
							l := bytecode.List{}
							s := in.NamedSpan(string(action.Variables[0]))
							for n := s.Start; n < s.Start+s.Length; n++ {
								var v any = big.NewInt(0)
								switch s.Dtype {
								case INT:
									v = in.V.Ints[n]
								}
								ListAppend(&l, in, v)
							}
							in.Save(action.Target, l)
						}
					}
				}
			case "value":
				err := in.CheckArgN(action, 1, 1) && in.CheckDtype(action, 0, ID)
				if err {
					in.Error(action, "error retrieving data from provided id", "id")
					return true
				}
				id := in.GetAny(string(action.Variables[0])).(uint64)
				if len(in.V.Slots) <= int(id) {
					in.Error(action, "invalid value id: higher than available memory", "id")
					return true
				}
				in.Save(action.Target, in.GetAnyRef(id))
			case "len":
				switch in.V.Slots[in.V.Names[string(action.Variables[0])]].Type {
				case STR:
					in.Save(action.Target, big.NewInt(int64(len([]rune(in.NamedStr(string(action.Variables[0])))))))
				case LIST:
					in.Save(action.Target, big.NewInt(int64(len(in.NamedList(string(action.Variables[0])).Ids))))
				case SPAN:
					s := in.NamedSpan(string(action.Variables[0]))
					in.Save(action.Target, big.NewInt(int64(s.Length)))
				}
			case "range":
				i := in.NamedInt(string(action.Variables[0]))
				s := in.NewSpan(int(i.Int64()), INT)
				iterated := big.NewInt(0)
				for iterated.Cmp(i) == -1 {
					in.V.Ints[s.Start+iterated.Uint64()].Set(iterated)
					iterated = big.NewInt(iterated.Int64() + 1)
				}
				in.Save(action.Target, s)
			case "list":
				l := bytecode.List{}
				for _, variable := range action.Variables {
					ListAppend(&l, in, in.GetAny(string(variable)))
				}
				in.Save(actions[focus].Target, l)
			case "input":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return err
				}
				RL.SetPrompt(in.NamedStr(string(action.Variables[0])))
				str, err_ := RL.Readline()
				if err_ != nil {
					in.Error(action, "keyboard interrupt!", "interrupt")
					return true
				}
				in.Save(action.Target, str)
			case "system":
				err := in.CheckArgN(actions[focus], 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(actions[focus], 0, STR)
				if err {
					return true
				}
				switch in.NamedStr(string(actions[focus].Variables[0])) {
				case "os":
					in.Save(actions[focus].Target, runtime.GOOS)
				case "version":
					in.Save(actions[focus].Target, "4.2.3")
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
			case "id":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return err
				}
				in.Save(action.Target, uint64(in.V.Names[string(action.Variables[0])]))
			case "append":
				l := in.NamedList(string(action.Variables[0]))
				ListAppend(&l, in, in.GetAny(string(action.Variables[1])))
				in.Save(action.Target, l)
			case "type":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return err
				}
				in.Save(action.Target, map[byte]string{NOTH: "noth", INT: "int", FLOAT: "float", BYTE: "byte", STR: "str", FUNC: "func", SPAN: "span", ID: "id", LIST: "list", BOOL: "bool", PAIR: "pair", ARR: "arr"}[in.V.Slots[in.V.Names[string(action.Variables[0])]].Type])
			default:
				if fn.Node != "" {
					// user functions start
					f_in := Interpreter{V: &Vars{
						Names: make(map[string]int),
					}}
					f_in.Copy(in)
					for n, fn_arg := range fn.Vars {
						fn_arg_str := string(fn_arg)
						if in.V.Slots[in.V.Names[string(action.Variables[n])]].Type == PAIR {
							p := f_in.CopyPair(string(action.Variables[n]), in)
							f_in.Save(fn_arg_str, p)
						} else if in.V.Slots[in.V.Names[string(action.Variables[n])]].Type == LIST {
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
					if f_in.V.Slots[f_in.V.Names["_return_"]].Type == LIST {
						l := in.CopyList("_return_", &f_in)
						in.Save(action.Target, l)
					} else if f_in.V.Slots[f_in.V.Names["_return_"]].Type == PAIR {
						l := in.CopyPair("_return_", &f_in)
						in.Save(action.Target, l)
					} else {
						in.Save(action.Target, f_in.GetAny("_return_"))
					}
					f_in.Destroy()
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

func chunkBy[T any](items []T, chunkSize int) (chunks [][]T) {
	for chunkSize < len(items) {
		items, chunks = items[chunkSize:], append(chunks, items[0:chunkSize:chunkSize])
	}
	return append(chunks, items)
}

func ChunkSliceIntoN[T any](s []T, n int) [][]T {
	if n <= 0 {
		return nil // Cannot split into zero or negative chunks
	}
	if len(s) == 0 {
		return [][]T{} // Return empty slice of slices for an empty input
	}
	if n >= len(s) {
		// If more chunks than elements, each element becomes its own chunk
		chunks := make([][]T, len(s))
		for i, v := range s {
			chunks[i] = []T{v}
		}
		return chunks
	}

	chunkSize := (len(s) + n - 1) / n // Calculate chunk size, rounding up
	chunks := make([][]T, 0, n)

	for i := 0; i < len(s); i += chunkSize {
		end := i + chunkSize
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}
	return chunks
}

// sourcesNames: list of variable names (in worker interpreter) that hold the chunks
func PoolWorkerNew(sourcesNames []string, node_name string, input_rights, output_lefts, output_rights []string, channel chan []bytecode.List, in2 *Interpreter) {
	in2.V.gcCycle++
	// Prepare output lists in worker (one per output_left)
	output := make([]bytecode.List, len(output_lefts))
	for i := range output {
		output[i] = bytecode.List{}
	}

	// Resolve the chunk lists inside the worker by name (these were saved into the worker by the parent)
	chunks := make([]bytecode.List, len(sourcesNames))
	for i, name := range sourcesNames {
		chunks[i] = in2.NamedList(name) // local to in2
	}

	// If no data, return empty outputs immediately
	if len(chunks) == 0 || len(chunks[0].Ids) == 0 {
		channel <- output
		return
	}

	// Iterate over the index range of the first chunk (assume equal-length inputs)
	for idx := range chunks[0].Ids {
		// Bind input variables inside worker to corresponding elements
		for m, c := range chunks {
			// c.Ids[idx] is a slot id local to in2, because we saved the chunk into in2 earlier
			valRef := in2.GetAnyRef(c.Ids[idx])
			in2.Save(input_rights[m], valRef)
		}

		// Run the worker's node (body)
		if err := in2.Run(node_name); err {
			// on error: skip or log (worker-local errors don't propagate here)
			continue
		}

		// Collect outputs from worker variables and append to output lists
		for m := range output_lefts {
			outVal := in2.GetAny(output_rights[m])
			ListAppend(&output[m], in2, outVal)
		}
	}

	// Send result lists (IDs are local to in2) back to main thread
	channel <- output
	println("completed")
}

func PoolWorkerNewLegacy(
	sources []bytecode.List,
	node_name string,
	input_rights, output_lefts, output_rights []string,
	channel chan []bytecode.List,
	in2 *Interpreter,
) {
	in2.V.gcCycle++
	output := make([]bytecode.List, len(output_lefts))

	// Iterate through all items in the first list (assuming equal length)
	if len(sources) == 0 {
		channel <- output
		return
	}

	//println("WORKER")
	for idx := range sources[0].Ids {
		// Bind inputs
		for m, src := range sources {
			//fmt.Println(input_rights[m], in2.V.Names)
			in2.Save(input_rights[m], in2.GetAnyRef(src.Ids[idx]))
		}

		err := in2.Run(node_name)
		if err {
			// TODO: proper error handling
			continue
		}

		// Collect outputs
		for m := range output_lefts {
			ListAppend(&output[m], in2, in2.GetAny(output_rights[m]))
		}
	}

	channel <- output
}

// MAIN FUNCTION END

// UTIL FUNCTIONS
func (in *Interpreter) Copy(og *Interpreter) {
	in.IgnoreErr = og.IgnoreErr
	in.Code = og.Code
	in.File = og.File
	// TODO: verify if needed
	for _, fn := range bytecode.GenerateFuns() {
		if _, ok := in.V.Names[fn.Name]; ok {
			continue
		}
		in.Save(fn.Name, &fn)
	}
	for key := range og.V.Names {
		in.Save(key, og.GetAny(key))
	}
}

func (in *Interpreter) Copy2(og *Interpreter) {
	in.IgnoreErr = og.IgnoreErr
	in.Code = og.Code
	in.File = og.File
	// TODO: verify if needed
	for _, fn := range bytecode.GenerateFuns() {
		if _, ok := in.V.Names[fn.Name]; ok {
			continue
		}
		in.Save(fn.Name, &fn)
	}
	for key := range og.V.Names {
		switch og.V.Slots[og.V.Names[key]].Type {
		case SPAN:
			//todo
		case LIST:
			l := in.CopyList(key, og)
			in.Save(key, l)
		default:
			in.Save(key, og.GetAny(key)) // for primitive types
		}
	}
}

func (in *Interpreter) CopyDeep(og *Interpreter) {
	in.IgnoreErr = og.IgnoreErr
	in.Code = og.Code
	in.File = og.File

	// Load built-in functions
	for _, fn := range bytecode.GenerateFuns() {
		if _, ok := in.V.Names[fn.Name]; ok {
			continue
		}
		in.Save(fn.Name, &fn)
	}

	// Copy user variables deeply
	for key := range og.V.Names {
		val := og.GetAny(key)

		switch v := val.(type) {
		case bytecode.List:
			// Deep copy List contents
			newList := bytecode.List{}
			for _, id := range v.Ids {
				elem := og.GetAnyRef(id)
				ListAppend(&newList, in, elem)
			}
			in.Save(key, newList)

		case bytecode.Span:
			// Deep copy Span content by expanding to actual values
			newSpan := bytecode.Span{
				Dtype:  v.Dtype,
				Start:  0,
				Length: v.Length,
			}

			switch v.Dtype {
			case INT:
				for i := v.Start; i < v.Start+v.Length && int(i) < len(og.V.Ints); i++ {
					in.Save(fmt.Sprintf("_span_%s_%d", key, i), og.V.Ints[i])
				}
			case FLOAT:
				for i := v.Start; i < v.Start+v.Length && int(i) < len(og.V.Floats); i++ {
					in.Save(fmt.Sprintf("_span_%s_%d", key, i), og.V.Floats[i])
				}
			case STR:
				for i := v.Start; i < v.Start+v.Length && int(i) < len(og.V.Strs); i++ {
					in.Save(fmt.Sprintf("_span_%s_%d", key, i), og.V.Strs[i])
				}
			}
			in.Save(key, newSpan)

		case bytecode.Array:
			// Copy Array by value (safe since it’s just a struct of slices)
			/*
				arrCopy := bytecode.Array{
					Dtype:  v.Dtype,
					Values: append([]any(nil), v.Values...),
				}
				in.Save(key, arrCopy)
			*/

		case bytecode.Pair:
			// Deep copy Pair contents
			newPair := bytecode.Pair{Ids: make(map[string]uint64)}
			for name, id := range v.Ids {
				elem := og.GetAnyRef(id)
				PairAppend(&newPair, in, elem, name)
				//ListAppendToPair(&newPair, in, name, elem)
			}
			in.Save(key, newPair)

		default:
			// Primitive values (ints, floats, bools, etc.)
			in.Save(key, val)
		}
	}
}

func (in *Interpreter) CopyList(vname string, og *Interpreter) bytecode.List {
	l := og.GetAny(vname).(bytecode.List)
	lnew := bytecode.List{}
	for _, id := range l.Ids {
		a := og.GetAnyRef(id)
		newid := in.SaveRefNew(a)
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
		newid := in.SaveRefNew(a)
		lnew.Ids[key] = newid
	}
	return lnew
}

func (in *Interpreter) Destroy() {
	//for key := range in.V.Names {
	//	in.RemoveName(key)
	//}

	in.GC()
	in = &Interpreter{}
}
