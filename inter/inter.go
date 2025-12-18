package inter

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"math/rand/v2"
	"minimum/bytecode"
	"minimum/input"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
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
	Ids     []*bytecode.MinPtr
	Arrs    []bytecode.Array
	Spans   []bytecode.Span
	Lists   []bytecode.List
	Pairs   []bytecode.Pair
	gcCycle uint16
	gcMax   uint16
	gcSize  uint64
}

func (v *Vars) heapSize() int {
	return len(v.Ints) +
		len(v.Floats) +
		len(v.Strs) +
		len(v.Bools) +
		len(v.Bytes) +
		len(v.Funcs) +
		len(v.Ids) +
		len(v.Arrs) +
		len(v.Spans) +
		len(v.Lists) +
		len(v.Pairs)
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
	Id        uint64
	Processes []*ChildProcess
}

type ChildProcess struct {
	Interp     *Interpreter
	Done       chan struct{} // signals completion
	ResultId   uint64        // ID of produced result (the “item”)
	ResultName string
	TargetList string // variable name in parent to append into
}

func (in *Interpreter) SpawnProcess(node string, targetList, childItem string, copies []string) {
	child := &Interpreter{V: &Vars{
		Names: make(map[string]int),
	}}
	child.Id = rand.Uint64()
	child.Copy(in)
	for _, c := range copies {
		child.Save(c, in.GetAny(c))
	}

	proc := &ChildProcess{
		Interp:     child,
		Done:       make(chan struct{}),
		TargetList: targetList,
		ResultName: childItem,
	}

	in.Processes = append(in.Processes, proc)

	// run concurrently
	go func() {
		child.Run(node)
		close(proc.Done)
	}()
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
	case *bytecode.MinPtr:
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
			case ID:
				in.V.Ids[in.V.Slots[old_id].Index] = v.(*bytecode.MinPtr)
			case ARR:
				in.V.Arrs[in.V.Slots[old_id].Index] = v.(bytecode.Array)
			case SPAN:
				in.V.Spans[in.V.Slots[old_id].Index] = v.(bytecode.Span)
			case FUNC:
				in.V.Funcs[in.V.Slots[old_id].Index] = v.(*bytecode.Function)
			case NOTH:
				in.Nothing(name)
			case PAIR:
				in.V.Pairs[in.V.Slots[old_id].Index] = v.(bytecode.Pair)
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
	case *bytecode.MinPtr:
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
	case int16:
		in.Nothing(name)
		return
	}
	in.V.Names[name] = len(in.V.Slots)
	in.V.Slots = append(in.V.Slots, entry)
}

func (in *Interpreter) SaveRef(old_id *bytecode.MinPtr, v any) {
	if TypeToByte(v) == in.V.Slots[old_id.Addr].Type {
		// value reassignment
		switch in.V.Slots[old_id.Addr].Type {
		case INT:
			in.V.Ints[in.V.Slots[old_id.Addr].Index] = v.(*big.Int)
		case FLOAT:
			in.V.Floats[in.V.Slots[old_id.Addr].Index] = v.(*big.Float)
		case STR:
			in.V.Strs[in.V.Slots[old_id.Addr].Index] = v.(string)
		case BYTE:
			in.V.Bytes[in.V.Slots[old_id.Addr].Index] = v.(byte)
		case BOOL:
			in.V.Bools[in.V.Slots[old_id.Addr].Index] = v.(bool)
		case LIST:
			in.V.Lists[in.V.Slots[old_id.Addr].Index] = v.(bytecode.List)
		case ARR:
			in.V.Arrs[in.V.Slots[old_id.Addr].Index] = v.(bytecode.Array)
		case SPAN:
			in.V.Spans[in.V.Slots[old_id.Addr].Index] = v.(bytecode.Span)
		case FUNC:
			in.V.Funcs[in.V.Slots[old_id.Addr].Index] = v.(*bytecode.Function)
		case NOTH:
			// TODO
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
	case *bytecode.MinPtr:
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
	case int16:
		// TODO: handle Nothing
	}
	/*
		name := ""
		for key, value := range in.V.Names {
			if old_id.Id == uint64(value) {
				name = key
				break
			}
		}
		if name != "" {
			in.V.Names[name] = len(in.V.Slots)
		}
	*/
	in.V.Slots = append(in.V.Slots, entry)
}

func (in *Interpreter) SaveRefNew(v any) *bytecode.MinPtr {
	entry := Entry{TypeToByte(v), 0}
	new_ref := &bytecode.MinPtr{0, in.Id}
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
	case *bytecode.MinPtr:
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
	new_ref.Addr = uint64(len(in.V.Slots))
	in.V.Slots = append(in.V.Slots, entry)
	return new_ref
}

func (in *Interpreter) GetAny(var_name string) any {
	switch in.Type(var_name) {
	case INT:
		return in.NamedInt(var_name)
	case FLOAT:
		return in.NamedFloat(var_name)
	case STR:
		return in.NamedStr(var_name)
	case LIST:
		return in.NamedList(var_name)
	case BOOL:
		return in.NamedBool(var_name)
	case BYTE:
		return in.NamedByte(var_name)
	case ARR:
		return in.NamedArr(var_name)
	case FUNC:
		return in.NamedFunc(var_name)
	case SPAN:
		return in.NamedSpan(var_name)
	case ID:
		return in.NamedId(var_name)
	case PAIR:
		return in.NamedPair(var_name)
	case NOTH:
		return int16(0)
	}
	return nil
}

func (in *Interpreter) GetAnyRef(ref *bytecode.MinPtr) any {
	if in.Id != ref.Id { // id test
		return in.Parent.GetAnyRef(ref)
	}
	ind := in.V.Slots[ref.Addr].Index
	switch in.V.Slots[ref.Addr].Type {
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
	case NOTH:
		return int16(0)
	}
	return nil
}

func PairKey(in *Interpreter, iname any) string {
	key_ref := in.GetRef(iname)
	// TODO: update dtype
	dtype := map[byte]string{0: "noth", 1: "int", 2: "float", 3: "str", 4: "arr", 5: "list", 6: "pair", 7: "bool", 8: "byte", 9: "func", 10: "id"}
	var iname_str string
	switch in.V.Slots[key_ref.Addr].Type { //TODO: add all types
	case INT:
		iname_str = in.V.Ints[in.V.Slots[key_ref.Addr].Index].String()
	case FLOAT:
		iname_str = in.V.Floats[in.V.Slots[key_ref.Addr].Index].String()
	case BYTE:
		iname_str = fmt.Sprintf("b.%d", in.V.Bytes[in.V.Slots[key_ref.Addr].Index])
	case BOOL:
		iname_str = ternary(in.V.Bools[in.V.Slots[key_ref.Addr].Index], "true", "false")
	case FUNC:
		iname_str = fmt.Sprintf("func.%s", in.V.Funcs[in.V.Slots[key_ref.Addr].Index].Name)
	case LIST:
		l := in.V.Lists[in.V.Slots[key_ref.Addr].Index]
		iname_str = ListString(&l, in)
	case PAIR:
		p := in.V.Pairs[in.V.Slots[key_ref.Addr].Index]
		iname_str = PairString(&p, in)
	case STR:
		iname_str = in.V.Strs[in.V.Slots[key_ref.Addr].Index]
	}
	key := fmt.Sprintf("%s:%s", dtype[in.V.Slots[key_ref.Addr].Type], iname_str)
	return key
}

func NewInterpreter(code, file string) Interpreter {
	in := Interpreter{}
	in.Id = rand.Uint64()
	in.Code = bytecode.GetCode(code)
	in.File = &file
	in.V = &Vars{}
	in.V.Names = make(map[string]int)
	for _, fn := range bytecode.GenerateFuns() {
		in.Save(fn.Name, &fn)
	}
	in.V.gcMax = 10000
	in.Nothing("Nothing") // create Nothing in top scope
	return in
}

func NewInterpreterPtr(code, file string) *Interpreter {
	in := &Interpreter{}
	in.Id = rand.Uint64()
	in.Code = bytecode.GetCode(code)
	in.File = &file
	in.V = &Vars{}
	in.V.Names = make(map[string]int)
	for _, fn := range bytecode.GenerateFuns() {
		in.Save(fn.Name, &fn)
	}
	in.V.gcMax = 10000
	in.Nothing("Nothing") // create Nothing in top scope
	return in
}

// this function returns a new interpreter with a (possibly) unique id
func NewInterId(code, file string, parent *Interpreter) *Interpreter {
	inter_id := rand.Uint64()
	in := Interpreter{}
	in.Id = inter_id
	if len(code) > 0 {
		in.Code = bytecode.GetCode(code)
		in.File = &file
	} else if parent != nil {
		in.Code = parent.Code
		in.File = parent.File
		in.Parent = parent
		in.IgnoreErr = parent.IgnoreErr
		in.ErrSource = parent.ErrSource
		in.V.gcCycle = parent.V.gcCycle
	}
	in.V = &Vars{}
	in.V.Names = make(map[string]int)
	if parent == nil {
		for _, fn := range bytecode.GenerateFuns() {
			in.Save(fn.Name, &fn)
		}
	}
	in.V.gcMax = 100
	return &in
}

func (in *Interpreter) EqualizeTypes(v1, v2 string) (string, string) { // returns tempvar names
	t1, t2 := in.Type(v1), in.Type(v2)
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

func (in *Interpreter) GetAnySlot(ptr *bytecode.MinPtr) Entry {
	if in.Id != ptr.Id {
		return in.Parent.GetAnySlot(ptr)
	}
	return in.V.Slots[ptr.Addr]
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
				oldEntry := in.GetAnySlot(slot) // old.Slots[slot.Addr]
				newSlot := copyEntry(oldEntry)
				newptr := &bytecode.MinPtr{uint64(len(newVars.Slots)), in.Id}
				newList.Ids = append(newList.Ids, newptr)
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
			newPair.Ids = make(map[string]*bytecode.MinPtr)
			for key, slot := range val.Ids {
				oldEntry := old.Slots[slot.Addr]
				newSlot := copyEntry(oldEntry)
				// old version:
				//newPair.Ids = append(newPair.Ids, uint64(len(newVars.Slots)))
				newPair.Ids[key] = &bytecode.MinPtr{uint64(len(newVars.Slots)), in.Id}
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
		if newSlot, ok := slotMap[int(oldSlot.Addr)]; ok {
			newVars.Ids[i] = &bytecode.MinPtr{uint64(newSlot), in.Id}
		} else {
			// Clear invalid or collected references
			newVars.Ids[i] = nil
		}
	}

	newVars.gcCycle = old.gcCycle
	newVars.gcMax = old.gcMax
	in.V = newVars
	runtime.GC()
}

// Garbage Collector Experimental
func (in *Interpreter) GCE() {
	in.V.gcCycle++
	heap_size := in.V.heapSize()
	if in.V.gcCycle >= in.V.gcMax {
		in.V.gcCycle = 0
	} else {
		if float64(in.V.gcSize)*1.2 < float64(heap_size) {
			in.V.gcCycle = 0
		} else {
			return
		}
	}
	old := in.V
	newVars := &Vars{
		Names:  make(map[string]int),
		gcSize: uint64(heap_size),
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
	slotMap := map[int]int{} // maps old slot index -> newVars.Slots index

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
			// copy the pointer struct; we'll remap its Addr later if needed
			oldPtr := old.Ids[e.Index]
			newIndex := len(newVars.Ids)
			// store the same pointer for now (we'll fix local references later)
			if oldPtr != nil {
				copied := &bytecode.MinPtr{Addr: oldPtr.Addr, Id: oldPtr.Id}
				newVars.Ids = append(newVars.Ids, copied)
			} else {
				newVars.Ids = append(newVars.Ids, nil)
			}
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
			case FLOAT:
				newSpan.Start = uint64(len(newVars.Floats))
				newVars.Floats = append(newVars.Floats, old.Floats[val.Start:val.Start+val.Length]...)
			case STR:
				newSpan.Start = uint64(len(newVars.Strs))
				newVars.Strs = append(newVars.Strs, old.Strs[val.Start:val.Start+val.Length]...)
			case BOOL:
				newSpan.Start = uint64(len(newVars.Bools))
				newVars.Bools = append(newVars.Bools, old.Bools[val.Start:val.Start+val.Length]...)
			case BYTE:
				newSpan.Start = uint64(len(newVars.Bytes))
				newVars.Bytes = append(newVars.Bytes, old.Bytes[val.Start:val.Start+val.Length]...)
			case ID:
				newSpan.Start = uint64(len(newVars.Ids))
				// copy Id pointers (we'll remap local ones later)
				for i := uint64(0); i < val.Length; i++ {
					if old.Ids[val.Start+i] != nil {
						ptr := &bytecode.MinPtr{Addr: old.Ids[val.Start+i].Addr, Id: old.Ids[val.Start+i].Id}
						newVars.Ids = append(newVars.Ids, ptr)
					} else {
						newVars.Ids = append(newVars.Ids, nil)
					}
				}
			default:
				// If unknown dtype, just leave span with zero start (safe fallback)
				newSpan.Start = 0
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
			for _, ptr := range val.Ids {
				if ptr == nil {
					// preserve nil
					newList.Ids = append(newList.Ids, nil)
					continue
				}
				// if the pointer refers to the same interpreter, copy the referenced slot into newVars
				if ptr.Id == in.Id {
					// get the old entry from old.Slots
					oldEntry := old.Slots[ptr.Addr]
					newSlotIdx := copyEntry(oldEntry)
					// new slot index in newVars.Slots is current length
					newSlotAddr := len(newVars.Slots)
					// append the slot entry
					newVars.Slots = append(newVars.Slots, Entry{Type: oldEntry.Type, Index: newSlotIdx})
					// record mapping from old slot to new slot
					slotMap[int(ptr.Addr)] = newSlotAddr
					// create pointer into newVars (local interpreter id)
					newList.Ids = append(newList.Ids, &bytecode.MinPtr{Addr: uint64(newSlotAddr), Id: in.Id})
				} else {
					// external pointer: preserve as-is (copy the struct)
					newList.Ids = append(newList.Ids, &bytecode.MinPtr{Addr: ptr.Addr, Id: ptr.Id})
				}
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
			newPair.Ids = make(map[string]*bytecode.MinPtr)
			for key, ptr := range val.Ids {
				if ptr == nil {
					newPair.Ids[key] = nil
					continue
				}
				if ptr.Id == in.Id {
					oldEntry := old.Slots[ptr.Addr]
					newSlotIdx := copyEntry(oldEntry)
					newSlotAddr := len(newVars.Slots)
					newVars.Slots = append(newVars.Slots, Entry{Type: oldEntry.Type, Index: newSlotIdx})
					slotMap[int(ptr.Addr)] = newSlotAddr
					newPair.Ids[key] = &bytecode.MinPtr{Addr: uint64(newSlotAddr), Id: in.Id}
				} else {
					// preserve external pointer
					newPair.Ids[key] = &bytecode.MinPtr{Addr: ptr.Addr, Id: ptr.Id}
				}
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
		newSlotIndex := len(newVars.Slots) // new slot index
		newVars.Names[name] = newSlotIndex
		newVars.Slots = append(newVars.Slots, Entry{Type: oldEntry.Type, Index: newIndex})
		slotMap[slotIdx] = newSlotIndex
	}

	// Update all entries in newVars.Ids to reflect updated slot mappings for local pointers.
	// Preserve pointers that point into other interpreters; set to nil when the local target was collected.
	for i := range newVars.Ids {
		// Corresponding original pointer was old.Ids[i] (copyEntry copied them in same index order)
		var oldPtr *bytecode.MinPtr
		if i < len(old.Ids) {
			oldPtr = old.Ids[i]
		} else {
			oldPtr = nil
		}
		if oldPtr == nil {
			newVars.Ids[i] = nil
			continue
		}
		// If pointer pointed into this interpreter, remap Addr if we copied that slot.
		if oldPtr.Id == in.Id {
			if newSlot, ok := slotMap[int(oldPtr.Addr)]; ok {
				newVars.Ids[i] = &bytecode.MinPtr{Addr: uint64(newSlot), Id: in.Id}
			} else {
				// target was collected / not reachable -> clear
				newVars.Ids[i] = nil
			}
		} else {
			// external pointer: keep original Addr/Id
			newVars.Ids[i] = &bytecode.MinPtr{Addr: oldPtr.Addr, Id: oldPtr.Id}
		}
	}

	// copy GC counters
	newVars.gcCycle = old.gcCycle
	newVars.gcMax = old.gcMax
	in.V = newVars
	runtime.GC()
}

func (in *Interpreter) Compile(code, fname string) {
	in2 := Interpreter{V: &Vars{
		Names: make(map[string]int),
	}}
	in2.Id = rand.Uint64()
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

var error_type, error_message, error_action string

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
	error_action = act.Type
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
	dstrings := map[byte]string{0: "noth", 1: "int", 2: "float", 3: "str", 4: "arr", 5: "list", 6: "pair", 7: "bool", 8: "byte", 9: "func", 10: "id", SPAN: "span"}
	found := false
	for _, dtype := range dtypes {
		// in.V.Slots[in.V.Names[string(action.Variables[index])]].Type
		if in.Type(string(action.Variables[index])) == dtype {
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

func (in *Interpreter) GetRef(v any) *bytecode.MinPtr {
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
	case *bytecode.MinPtr:
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
	return &bytecode.MinPtr{uint64(value), in.Id}
}

func ListAppend(l *bytecode.List, in *Interpreter, item any) {
	ref := in.GetRef(item)
	l.Ids = append(l.Ids, ref)
}

func ListString(l *bytecode.List, in *Interpreter) string {
	elements := []string{}
	for _, item_id := range l.Ids {
		interp := in
		for interp.Id != item_id.Id {
			interp = interp.Parent
		}
		item := interp.V.Slots[item_id.Addr].Index
		switch in.TypeRef(item_id) {
		case INT:
			elements = append(elements, interp.V.Ints[item].String())
		case FLOAT:
			elements = append(elements, interp.V.Floats[item].String())
		case BOOL:
			elements = append(elements, ternary(interp.V.Bools[item], "true", "false"))
		case BYTE:
			elements = append(elements, fmt.Sprintf("b.%d", interp.V.Bytes[item]))
		case STR:
			elements = append(elements, "\""+interp.V.Strs[item]+"\"")
		case FUNC:
			elements = append(elements, "func."+interp.V.Funcs[item].Name)
		case LIST:
			ll := in.V.Lists[item]
			elements = append(elements, ListString(&ll, interp))
		case PAIR:
			pp := in.V.Pairs[item]
			elements = append(elements, PairString(&pp, interp))
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
		switch in.V.Slots[item.Addr].Type {
		case INT:
			elements = append(elements, dkey+": "+in.V.Ints[in.V.Slots[item.Addr].Index].String())
		case FLOAT:
			elements = append(elements, dkey+": "+in.V.Floats[in.V.Slots[item.Addr].Index].String())
		case BOOL:
			elements = append(elements, dkey+": "+ternary(in.V.Bools[in.V.Slots[item.Addr].Index], "true", "false"))
		case BYTE:
			elements = append(elements, dkey+": "+fmt.Sprintf("b.%d", in.V.Bytes[in.V.Slots[item.Addr].Index]))
		case STR:
			elements = append(elements, dkey+": \""+in.V.Strs[in.V.Slots[item.Addr].Index]+"\"")
		case LIST:
			ll := in.V.Lists[in.V.Slots[item.Addr].Index]
			elements = append(elements, dkey+": "+ListString(&ll, in))
		case FUNC:
			elements = append(elements, dkey+": "+"func."+in.V.Funcs[in.V.Slots[item.Addr].Index].Name)
		case PAIR:
			pp := in.V.Pairs[in.V.Slots[item.Addr].Index]
			elements = append(elements, dkey+": "+PairString(&pp, in))
		}
	}
	return "{" + strings.Join(elements, ", ") + "}"
}

// GETTING VARIABLES START
// return in.V.Ints[in.V.Slots[in.V.Names[vname]].Index]
func (in *Interpreter) NamedInt(vname string) *big.Int {
	if slot_index, ok := in.V.Names[vname]; ok {
		return in.V.Ints[in.V.Slots[slot_index].Index]
	} else {
		if in.Parent != nil {
			return in.Parent.NamedInt(vname)
		} else {
			return big.NewInt(0)
		}
	}
}
func (in *Interpreter) NamedFloat(vname string) *big.Float {
	if slot_index, ok := in.V.Names[vname]; ok {
		return in.V.Floats[in.V.Slots[slot_index].Index]
	} else {
		if in.Parent != nil {
			return in.Parent.NamedFloat(vname)
		} else {
			return big.NewFloat(0)
		}
	}
}
func (in *Interpreter) NamedStr(vname string) string {
	if slot_index, ok := in.V.Names[vname]; ok {
		return in.V.Strs[in.V.Slots[slot_index].Index]
	} else {
		if in.Parent != nil {
			return in.Parent.NamedStr(vname)
		} else {
			return ""
		}
	}
}
func (in *Interpreter) NamedBool(vname string) bool {
	if slot_index, ok := in.V.Names[vname]; ok {
		return in.V.Bools[in.V.Slots[slot_index].Index]
	} else {
		if in.Parent != nil {
			return in.Parent.NamedBool(vname)
		} else {
			return false
		}
	}
}
func (in *Interpreter) NamedByte(vname string) byte {
	if slot_index, ok := in.V.Names[vname]; ok {
		return in.V.Bytes[in.V.Slots[slot_index].Index]
	} else {
		if in.Parent != nil {
			return in.Parent.NamedByte(vname)
		} else {
			return 0
		}
	}
}
func (in *Interpreter) NamedFunc(vname string) *bytecode.Function {
	if slot_index, ok := in.V.Names[vname]; ok {
		return in.V.Funcs[in.V.Slots[slot_index].Index]
	} else {
		if in.Parent != nil {
			return in.Parent.NamedFunc(vname)
		} else {
			return &bytecode.Function{}
		}
	}
}
func (in *Interpreter) NamedList(vname string) bytecode.List {
	if slot_index, ok := in.V.Names[vname]; ok {
		return in.V.Lists[in.V.Slots[slot_index].Index]
	} else {
		if in.Parent != nil {
			return in.Parent.NamedList(vname)
		} else {
			return bytecode.List{}
		}
	}
}
func (in *Interpreter) NamedArr(vname string) bytecode.Array {
	if slot_index, ok := in.V.Names[vname]; ok {
		return in.V.Arrs[in.V.Slots[slot_index].Index]
	} else {
		if in.Parent != nil {
			return in.Parent.NamedArr(vname)
		} else {
			return bytecode.Array{}
		}
	}
}
func (in *Interpreter) NamedSpan(vname string) bytecode.Span {
	if slot_index, ok := in.V.Names[vname]; ok {
		return in.V.Spans[in.V.Slots[slot_index].Index]
	} else {
		if in.Parent != nil {
			return in.Parent.NamedSpan(vname)
		} else {
			return in.NewSpan(0, BYTE)
		}
	}
}
func (in *Interpreter) NamedPair(vname string) bytecode.Pair {
	if slot_index, ok := in.V.Names[vname]; ok {
		return in.V.Pairs[in.V.Slots[slot_index].Index]
	} else {
		if in.Parent != nil {
			return in.Parent.NamedPair(vname)
		} else {
			p := bytecode.Pair{}
			p.Ids = make(map[string]*bytecode.MinPtr)
			return p
		}
	}
}
func (in *Interpreter) NamedId(vname string) *bytecode.MinPtr {
	if slot_index, ok := in.V.Names[vname]; ok {
		return in.V.Ids[in.V.Slots[slot_index].Index]
	} else {
		if in.Parent != nil {
			return in.Parent.NamedId(vname)
		} else {
			return nil
		}
	}
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
		case FLOAT:
			s.Start = uint64(len(in.V.Floats))
			s.Length = uint64(length)
			for range length {
				in.V.Floats = append(in.V.Floats, big.NewFloat(0))
			}
		case BOOL:
			s.Start = uint64(len(in.V.Bools))
			s.Length = uint64(length)
			for range length {
				in.V.Bools = append(in.V.Bools, false)
			}
		case BYTE:
			s.Start = uint64(len(in.V.Bytes))
			s.Length = uint64(length)
			for range length {
				in.V.Bytes = append(in.V.Bytes, byte(0))
			}
		case FUNC:
			s.Start = uint64(len(in.V.Funcs))
			s.Length = uint64(length)
			for range length {
				in.V.Funcs = append(in.V.Funcs, &bytecode.Function{})
			}
		case LIST:
			s.Start = uint64(len(in.V.Lists))
			s.Length = uint64(length)
			for range length {
				in.V.Lists = append(in.V.Lists, bytecode.List{})
			}
		case SPAN:
			s.Start = uint64(len(in.V.Spans))
			s.Length = uint64(length)
			for range length {
				in.V.Spans = append(in.V.Spans, in.NewSpan(0, NOTH))
			}
		case PAIR:
			s.Start = uint64(len(in.V.Pairs))
			s.Length = uint64(length)
			for range length {
				p := bytecode.Pair{make(map[string]*bytecode.MinPtr)}
				in.V.Pairs = append(in.V.Pairs, p)
			}
		case NOTH:
			s.Start = uint64(0)
			s.Length = uint64(0)
		default:
			panic("unsupported Span content")
		}
		return s
	}
}

func (in *Interpreter) SpanSet(s *bytecode.Span, index int, item any) error {
	types := map[byte]string{NOTH: "noth", INT: "int", FLOAT: "float", BYTE: "byte", STR: "str", FUNC: "func", SPAN: "span", ID: "id", LIST: "list", BOOL: "bool", PAIR: "pair", ARR: "arr"}
	if index < 0 {
		index += int(s.Length)
	}
	if index < 0 || index >= int(s.Length) {
		return fmt.Errorf("impossible span index: %d", index)
	}
	switch v := item.(type) {
	case *big.Int:
		if s.Dtype == INT {
			in.V.Ints[s.Start+uint64(index)] = v
		} else {
			return fmt.Errorf("cannot append item with type code %s to span with type code %s", types[INT], types[s.Dtype])
		}
	case *big.Float:
		if s.Dtype == FLOAT {
			in.V.Floats[s.Start+uint64(index)] = v
		} else {
			return fmt.Errorf("cannot append item with type code %s to span with type code %s!", types[FLOAT], types[s.Dtype])
		}
	case byte:
		if s.Dtype == BYTE {
			in.V.Bytes[s.Start+uint64(index)] = v
		} else {
			return fmt.Errorf("cannot append item with type code %s to span with type code %s!", types[BYTE], types[s.Dtype])
		}
	case bool:
		if s.Dtype == BOOL {
			in.V.Bools[s.Start+uint64(index)] = v
		} else {
			return fmt.Errorf("cannot append item with type code %s to span with type code %s!", types[BOOL], types[s.Dtype])
		}
	case bytecode.Span:
		if s.Dtype == SPAN {
			in.V.Spans[s.Start+uint64(index)] = v
		} else {
			return fmt.Errorf("cannot append item with type code %s to span with type code %s!", types[SPAN], types[s.Dtype])
		}
	case bytecode.List:
		if s.Dtype == LIST {
			in.V.Lists[s.Start+uint64(index)] = v
		} else {
			return fmt.Errorf("cannot append item with type code %s to span with type code %s!", types[LIST], types[s.Dtype])
		}
	case bytecode.Pair:
		if s.Dtype == PAIR {
			in.V.Pairs[s.Start+uint64(index)] = v
		} else {
			return fmt.Errorf("cannot append item with type code %s to span with type code %s!", types[PAIR], types[s.Dtype])
		}
	case *bytecode.Function:
		if s.Dtype == FUNC {
			in.V.Funcs[s.Start+uint64(index)] = v
		} else {
			return fmt.Errorf("cannot append item with type code %s to span with type code %s!", types[FUNC], types[s.Dtype])
		}
	default:
		return fmt.Errorf("unsupported type!")
	}
	return nil
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

func (in *Interpreter) StringifyAny(original any, prefix, suffix int) string {
	str := ""
	switch value := original.(type) {
	case *big.Int:
		str = value.String()
		is_neg := false
		if strings.HasPrefix(str, "-") {
			is_neg = true
			str = str[1:]
		}
		for len(str) < prefix {
			str = "0" + str
		}
		str = ternary(is_neg, "-"+str, "+"+str) + "."
		for range suffix {
			str += "0"
		}
	case *big.Float:
		str = value.String()
		is_neg := false
		if strings.HasPrefix(str, "-") {
			is_neg = true
			str = str[1:]
		}
		for len(strings.Split(str, ".")[0]) < prefix {
			str = "0" + str
		}
		str = ternary(is_neg, "-"+str, "+"+str)
		for len(strings.Split(str, ".")[0]) < suffix {
			str += "0"
		}
	case string:
		str = value
	case bytecode.List:
		str = ListString(&value, in)
	}
	for prefix+suffix > len([]rune(str)) {
		str += string([]rune{0})
	}
	if !(strings.HasPrefix(str, "-") || strings.HasPrefix(str, "+")) {
		str = "+" + str
	}
	if !strings.Contains(str, ".") {
		str += "."
	}
	return str
}

type SortItem struct {
	Str  []rune
	Item any
}

func GreaterString(s0, s1 []rune) bool {
	for n := range s0 {
		if s0[n] < s1[n] {
			return false
		}
	}
	return true
}

func (in *Interpreter) Sort(items []any, ids []string) []any {
	array := []SortItem{}
	if ids == nil {
		pre, suf := 0, 0
		for _, item := range items {
			p := in.StringifyAny(item, 0, 0)
			sliced := strings.Split(p, ".")
			before := strings.Join(sliced[:len(sliced)-1], ".")
			after := sliced[len(sliced)-1]
			if length := len([]rune(before)); length > pre {
				pre = length
			}
			if length := len([]rune(after)); length > suf {
				suf = length
			}
		}
		ids = []string{}
		for _, item := range items {
			p := in.StringifyAny(item, pre, suf)
			ids = append(ids, p)
		}
	}
	for n := range items {
		array = append(array, SortItem{[]rune(ids[n]), items[n]})
	}
	pos0, pos1 := 0, 1
	for len(array) > 1 {
		if pos0 >= len(array) {
			break
		}
		if pos0 != pos1 && !GreaterString(array[pos0].Str, array[pos1].Str) {
			array[pos0], array[pos1] = array[pos1], array[pos0]
			pos0, pos1 = 0, 0
		}
		pos1++
		if pos1 >= len(array) {
			pos1 = 0
			pos0++
		}
	}
	sorted_items := []any{}
	for _, ai := range array {
		sorted_items = append(sorted_items, ai)
	}
	return sorted_items
}

func (in *Interpreter) SortList(l bytecode.List) (bytecode.List, error) {
	for {
		is_sorted, err := in.IsSortedList(l)
		if is_sorted {
			break
		} else {
			if err != nil {
				return bytecode.List{}, err
			}
			in.SortListStep(&l)
		}
	}
	return l, nil
}

func (in *Interpreter) IsSortedList(l bytecode.List) (bool, error) {
	for i := 0; i < len(l.Ids)-1; i++ {
		id0, id1 := l.Ids[i], l.Ids[i+1]
		less, err := in.CompareLess(id0, id1)
		if err != nil {
			return false, err
		}
		if !less {
			// If current element is not less than next, list is not sorted
			return false, nil
		}
	}
	return true, nil
}

func (in *Interpreter) CompareLess(id0, id1 *bytecode.MinPtr) (bool, error) {
	if bytecode.Has([]byte{INT, FLOAT, BYTE}, in.V.Slots[id0.Addr].Type) &&
		bytecode.Has([]byte{INT, FLOAT, BYTE}, in.V.Slots[id1.Addr].Type) {
		// Numeric comparison
		f0, f1 := big.NewFloat(0), big.NewFloat(0)
		switch in.V.Slots[id0.Addr].Type {
		case INT:
			f0.SetInt(in.GetAnyRef(id0).(*big.Int))
		case FLOAT:
			f0.Set(in.GetAnyRef(id0).(*big.Float))
		case BYTE:
			f0.SetFloat64(float64(in.GetAnyRef(id0).(byte)))
		}
		switch in.V.Slots[id1.Addr].Type {
		case INT:
			f1.SetInt(in.GetAnyRef(id1).(*big.Int))
		case FLOAT:
			f1.Set(in.GetAnyRef(id1).(*big.Float))
		case BYTE:
			f1.SetFloat64(float64(in.GetAnyRef(id1).(byte)))
		}
		return f0.Cmp(f1) < 0, nil

	} else if in.V.Slots[id0.Addr].Type == STR && in.V.Slots[id1.Addr].Type == STR {
		// String comparison
		s0 := in.GetAnyRef(id0).(string)
		s1 := in.GetAnyRef(id1).(string)
		return s0 < s1, nil

	} else if in.V.Slots[id0.Addr].Type == LIST && in.V.Slots[id1.Addr].Type == LIST {
		// List comparison - compare element by element
		l0, l1 := in.GetAnyRef(id0).(bytecode.List), in.GetAnyRef(id1).(bytecode.List)

		// Compare element by element up to the minimum length
		minLen := len(l0.Ids)
		if len(l1.Ids) < minLen {
			minLen = len(l1.Ids)
		}

		for i := 0; i < minLen; i++ {
			less, err := in.CompareLess(l0.Ids[i], l1.Ids[i])
			if err != nil {
				return false, err
			}
			greater, err := in.CompareLess(l1.Ids[i], l0.Ids[i])
			if err != nil {
				return false, err
			}

			// If elements are not equal, return the comparison result
			if less {
				return true, nil
			}
			if greater {
				return false, nil
			}
			// If equal, continue to next element
		}

		// If all compared elements are equal, the shorter list is "less"
		return len(l0.Ids) < len(l1.Ids), nil

	} else {
		type_map := map[byte]string{NOTH: "noth", INT: "int", FLOAT: "float", BYTE: "byte", STR: "str", FUNC: "func", SPAN: "span", ID: "id", LIST: "list", BOOL: "bool", PAIR: "pair", ARR: "arr"}
		return false, fmt.Errorf("impossible comparison in sort function: #%d (%s) against #%d (%s)", id0, type_map[in.V.Slots[id0.Addr].Type], id1, type_map[in.V.Slots[id1.Addr].Type])
	}
}

func (in *Interpreter) SortListStep(l *bytecode.List) error {
	for i := 0; i < len(l.Ids)-1; i++ {
		id0, id1 := l.Ids[i], l.Ids[i+1]
		less, err := in.CompareLess(id0, id1)
		if err != nil {
			return err
		}
		if !less {
			// If current element is not less than next, swap them
			l.Ids[i], l.Ids[i+1] = id1, id0
			return nil
		}
	}
	return nil
}

func (in *Interpreter) Parse(str string) []string {
	reg_var := regexp.MustCompile(`\{.+?\}`)
	for _, match := range reg_var.FindAllString(str, -1) {
		code := match[1 : len(match)-1]
		if variable, ok := in.V.Names[code]; ok {
			a := in.GetAnyRef(&bytecode.MinPtr{uint64(variable), in.Id})
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
			a := in.GetAnyRef(&bytecode.MinPtr{uint64(variable), in.Id})
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
			case *bytecode.Function:
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
			case *bytecode.Function:
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

func (in *Interpreter) Stringify(v any) string {
	str := "Nothing"
	switch vt := v.(type) {
	case *big.Int:
		str = vt.String()
	case *big.Float:
		str = vt.String()
	case string:
		str = vt
	case byte:
		str = fmt.Sprintf("b.%d", vt)
	case bool:
		str = ternary(vt, "true", "false")
	case bytecode.List:
		str = ListString(&vt, in)
	case bytecode.Span:
		str = in.StringSpan(vt)
	case *bytecode.Function:
		str = fmt.Sprintf("func.%s", vt.Name)
	case bytecode.Pair:
		str = PairString(&vt, in)
	case *bytecode.MinPtr:
		str = fmt.Sprintf("id.%x@%x", vt.Addr, vt.Id)
	}
	return str
}

var RL = input.Rl
var protected_actions = []string{"for", "const", "pool", "error", "func", "process"}

func (in *Interpreter) RunSort(ftype string, sl *bytecode.SourceLine) bool {
	node_name := fmt.Sprintf("_runner_%x", rand.Int64())
	target := fmt.Sprintf("_targ_%x", rand.Int64())
	arguments := []bytecode.Variable{("item")}
	in.Code[node_name] = []bytecode.Action{{Target: target, Type: ftype, Variables: arguments, Source: sl}}
	boolean := in.Run(node_name)
	delete(in.Code, node_name)
	return boolean
}

func (in *Interpreter) checkChildProcesses() {
	i := 0
	for i < len(in.Processes) {
		proc := in.Processes[i]

		select {
		case <-proc.Done:
			// child finished — integrate result
			in.integrateChildResult(proc)
			// remove from slice
			in.Processes = append(in.Processes[:i], in.Processes[i+1:]...)
		default:
			i++
		}
	}
}

func (in *Interpreter) integrateChildResult(proc *ChildProcess) {
	// append result into parent's list
	lst := in.NamedList(proc.TargetList)
	lst = in.CopyList(proc.TargetList, proc.Interp)
	ref := in.SaveRefNew(proc.Interp.GetAny(proc.ResultName))
	lst.Ids = append(lst.Ids, ref)
	in.Save(proc.TargetList, lst)
}

// MAIN FUNCTION START
func (in *Interpreter) Run(node_name string) bool {
	actions, ok := in.Code[node_name]
	if !ok && strings.HasPrefix(node_name, "action") {
		a := bytecode.Action{}
		a.Parse(node_name)
		actions = append(actions, a)
	}
	focus := 0
	for focus < len(actions) && !in.halt {
		in.checkChildProcesses()
		action := actions[focus]
		for _, vv := range action.Variables {
			if _, ok := in.V.Names[string(vv)]; !ok && !bytecode.Has(protected_actions, action.Type) {
				interp := in.Parent
				_, ok = interp.V.Names[string(vv)]
				for !ok && interp.Parent != nil {
					interp = interp.Parent
					_, ok = interp.V.Names[string(vv)]
				}
				if !ok {
					in.Error(action, "Undeclared variable: "+string(vv), "undeclared")
					return true
				}
			}
		}
		if action.Type != "++" && action.Type != "--" {
			in.Nothing(action.Target)
		} // TODO: check if creates bloat
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
			switch in.Type(action.First()) {
			case STR:
				str := in.NamedStr(string(action.Variables[0]))
				ind := in.NamedInt(string(action.Variables[1])).Int64()
				if ind < 0 {
					ind += int64(len([]rune(str)))
				}
				if action.Type == "'" {
					in.Save(action.Target, string([]rune(str)[ind]))
				}
			case PAIR:
				p := in.NamedPair(string(action.Variables[0]))
				ind := PairKey(in, in.GetAny(string(action.Variables[1])))
				if _, ok := p.Ids[ind]; !ok {
					in.Error(action, fmt.Sprintf("invalid pairing key: %s", strings.SplitN(ind, ":", 2)[1]), "index")
					return true
				}
				if action.Type == "'" {
					in.Save(action.Target, in.GetAnyRef(p.Ids[ind]))
				} else {
					in.V.Names[actions[focus].Target] = int(p.Ids[ind].Addr)
				}
			case LIST:
				item, go_err := in.IndexList(in.NamedList(action.First()), in.GetAny(action.Second()))
				if go_err != nil {
					in.Error(action, go_err.Error(), "index")
					return true
				}
				in.Save(action.Target, item)
				// TODO: add errors and slice support
				/*
					l := in.NamedList(string(action.Variables[0]))
					ind := in.NamedInt(string(action.Variables[1])).Int64()
					if ind < 0 {
						ind += int64(len(l.Ids))
					}
					if action.Type == "'" {
						in.Save(action.Target, in.GetAnyRef(l.Ids[ind]))
					} else {
						in.V.Names[action.Target] = int(l.Ids[ind].Addr)
					}
				*/
			}
		case "deep":
			in.Save(string(action.Variables[0]), in.GetAny(string(action.Variables[1])))
		case "sub":
			merr := in.CheckArgN(action, 3, -1)
			if merr {
				return merr
			}
			merr = in.CheckDtype(action, 0, LIST, PAIR)
			if in.Type(action.First()) == LIST {
				l := in.NamedList(action.First())
				inds := []any{}
				for _, ind := range action.Variables[2:] {
					inds = append(inds, in.GetAny(string(ind)))
				}
				err := in.DeepAssign(&l, in.GetAny(action.Second()), inds)
				if err != nil {
					in.Error(action, err.Error(), "index")
					return true
				}
				in.Save(action.Target, l)
			} else {
				l := in.NamedPair(action.First())
				inds := []any{}
				for _, ind := range action.Variables[2:] {
					inds = append(inds, in.GetAny(string(ind)))
				}
				err := in.DeepAssign(&l, in.GetAny(action.Second()), inds)
				if err != nil {
					in.Error(action, err.Error(), "index")
					return true
				}
				in.Save(action.Target, l)
			}
		case "pair":
			p := bytecode.Pair{}
			p.Ids = make(map[string]*bytecode.MinPtr)
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
				f_in.Id = rand.Uint64()
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
							if _, ok := win.V.Names[name]; !ok {
								win.Nothing(name)
							}
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
				f_in.Id = rand.Uint64()
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
			// in.V.Names[action.Target] = in.V.Names[string(actions[focus].Variables[0])]
			interp := in
			_, ok := interp.V.Names[action.Target]
			for !ok {
				interp = interp.Parent
				if interp == nil {
					in.Error(action, "undeclared variable name", "undeclared")
					return true
				}
				_, ok = interp.V.Names[action.Target]
			}
			interp.Save(action.Target, in.GetAny(action.First()))
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
			in.Save(action.Target, in.CompareName(o, t))
		case "!=":
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), string(actions[focus].Variables[1]))
			in.Save(action.Target, !in.CompareName(o, t))
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
		case "and":
			err := in.CheckDtype(action, 0, BOOL)
			if err {
				return true
			}
			err = in.CheckDtype(action, 1, BOOL)
			if err {
				return true
			}
			in.Save(action.Target, in.NamedBool(string(action.Variables[0])) && in.NamedBool(string(action.Variables[1])))
		case "or":
			err := in.CheckDtype(action, 0, BOOL)
			if err {
				return true
			}
			err = in.CheckDtype(action, 1, BOOL)
			if err {
				return true
			}
			in.Save(action.Target, in.NamedBool(string(action.Variables[0])) || in.NamedBool(string(action.Variables[1])))
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
			if len(action.Variables) > 0 {
				in.Save("_case"+action.Target, in.GetAny(string(action.Variables[0])))
			} else {
				in.Save("_case"+action.Target, true)
			}
			err := in.Run(action.Target)
			if err {
				return true
			}
			in.RemoveName("_case" + action.Target)
		case "case":
			if len(action.Variables) == 0 {
				err := in.Run(action.Target)
				return err
			}
			o, t := in.EqualizeTypes(string(actions[focus].Variables[0]), "_case"+node_name)
			switch in.Type(o) {
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
			case BOOL:
				if in.NamedBool(o) == in.NamedBool(t) {
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
						case PAIR:
							in.Save(targets[i], in.V.Pairs[valIndex])
						case LIST:
							in.Save(targets[i], in.V.Lists[valIndex])
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
						interp := in
						for interp.Id != valIndex.Id {
							interp = interp.Parent
						}
						switch interp.V.Slots[valIndex.Addr].Type {
						case INT:
							in.Save(targets[i], interp.V.Ints[interp.V.Slots[valIndex.Addr].Index])
						case FLOAT:
							in.Save(targets[i], interp.V.Floats[interp.V.Slots[valIndex.Addr].Index])
						case STR:
							in.Save(targets[i], interp.V.Strs[interp.V.Slots[valIndex.Addr].Index])
						case BOOL:
							in.Save(targets[i], interp.V.Bools[interp.V.Slots[valIndex.Addr].Index])
						case BYTE:
							in.Save(targets[i], interp.V.Bytes[interp.V.Slots[valIndex.Addr].Index])
						case ID:
							in.Save(targets[i], interp.V.Ids[interp.V.Slots[valIndex.Addr].Index])
						case LIST:
							in.Save(targets[i], interp.V.Lists[interp.V.Slots[valIndex.Addr].Index])
						case PAIR:
							in.Save(targets[i], interp.V.Pairs[interp.V.Slots[valIndex.Addr].Index])
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
		case "process":
			node := action.Target
			in.Save(action.Second(), bytecode.List{})
			frozen := []string{}
			for _, v := range action.Variables[2:] {
				frozen = append(frozen, string(v))
			}
			in.SpawnProcess(node, action.Second(), action.First(), frozen)
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
				p.Ids = make(map[string]*bytecode.MinPtr)
				PairAppend(&p, in, big.NewInt(int64(in.ErrSource.N)+1), "line")
				PairAppend(&p, in, in.ErrSource.Source, "source")
				PairAppend(&p, in, error_type, "action")
				PairAppend(&p, in, error_type, "type")
				PairAppend(&p, in, error_message, "message")
				error_type = ""
				error_message = ""
				in.Save(string(action.Variables[1]), p)
			}
			in.IgnoreErr = false
		case "except":
			e := in.CheckArgN(action, 2, 2)
			if e {
				return e
			}
			e = in.CheckDtype(action, 0, STR)
			if e {
				return e
			}
			e = in.CheckDtype(action, 1, STR)
			if e {
				return e
			}
			in.Error(action, in.NamedStr(action.First()), in.NamedStr(action.Second()))
			return true
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
			in.GCE()
		default:
			fn := in.NamedFunc(action.Type) //in.GetAny(action.Type).(*bytecode.Function) //in.V.Funcs[in.V.Names[actions[focus].Type]]
			// TODO: add boundcheck
			/*
				if !ok {
					in.Error(actions[focus], "Undeclared function!", "undeclared")
				}
			*/
			switch fn.Name {
			case "print", "out":
				for n, v := range action.Variables {
					fmt.Print(in.Stringify(in.GetAny(string(v))))
					if n != len(action.Variables)-1 {
						fmt.Print(" ")
					}
				}
				if action.Type == "print" {
					fmt.Println()
				}
			case "replace":
				err := in.CheckArgN(action, 3, 4)
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
				limit := -1
				if len(action.Variables) > 3 {
					err = in.CheckDtype(action, 3, INT)
					if err {
						return err
					}
					limit = int(in.NamedInt(string(action.Variables[3])).Int64())
				}
				in.Save(action.Target, strings.Replace(in.NamedStr(action.First()), in.NamedStr(action.Second()), in.NamedStr(action.Third()), limit))
			case "source":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return true
				}
				b, ferr := os.ReadFile(in.NamedStr(string(action.Variables[0])))
				if ferr != nil {
					in.Error(action, ferr.Error(), "sys")
					return true
				}
				in.Compile(string(b), in.NamedStr(string(action.Variables[0])))
				last_node := fmt.Sprintf("_node_%d", bytecode.NodeN-1)
				err = in.Run(last_node)
				if err {
					return err
				}
			case "run":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return true
				}
				c := in.NamedStr(action.First())
				in.Compile(c, "\""+c+"\"")
				last_node := fmt.Sprintf("_node_%d", bytecode.NodeN-1)
				err = in.Run(last_node)
				if err {
					return err
				}
			case "runf": // run & forget
				err := in.CheckArgN(action, 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return true
				}
				c := in.NamedStr(action.First())
				var nodes []string
				for node_name := range in.Code {
					nodes = append(nodes, node_name)
				}
				in.Compile(c, "\""+c+"\"")
				last_node := fmt.Sprintf("_node_%d", bytecode.NodeN-1)
				if len(in.Code[last_node]) > 1 {
					in.Code[last_node] = in.Code[last_node][:len(in.Code[last_node])-1] // let's remove GC action
				}
				err = in.Run(last_node)
				if err {
					return err
				}
				in.Save(action.Target, in.GetAny(in.Code[last_node][len(in.Code[last_node])-1].Target))
				for node_name := range in.Code {
					if !bytecode.Has(nodes, node_name) {
						delete(in.Code, node_name)
					}
				}
			case "isdir":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return true
				}
				is_dir, go_err := isDirectory(in.NamedStr(action.First()))
				if go_err != nil {
					in.Error(action, go_err.Error(), "sys")
					return true
				}
				in.Save(action.Target, is_dir)
			case "abs":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, STR, INT, FLOAT)
				if err {
					return true
				}
				if in.Type(action.First()) == STR {
					path := in.NamedStr(action.First())
					if filepath.IsAbs(path) {
						in.Save(action.Target, path)
					} else {
						path_abs, go_err := filepath.Abs(path)
						if go_err != nil {
							in.Error(action, go_err.Error(), "sys")
							return true
						}
						in.Save(action.Target, path_abs)
					}
				} else {
					switch in.Type(action.First()) {
					case INT:
						i := in.NamedInt(action.First())
						in.Save(action.Target, i.Abs(i))
					case FLOAT:
						i := in.NamedFloat(action.First())
						in.Save(action.Target, i.Abs(i))
					}
				}
			case "ternary":
				err := in.CheckArgN(action, 3, 3)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, BOOL)
				if err {
					return true
				}
				in.Save(action.Target, ternary(in.NamedBool(string(action.Variables[0])), in.GetAny(string(action.Variables[1])), in.GetAny(string(action.Variables[2]))))
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
			case "lower":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return true
				}
				in.Save(action.Target, strings.ToLower(in.NamedStr(action.First())))
			case "upper":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return true
				}
				in.Save(action.Target, strings.ToUpper(in.NamedStr(action.First())))
			case "map":
				err := in.CheckArgN(action, 2, 2)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, LIST)
				if err {
					return true
				}
				err = in.CheckDtype(action, 1, FUNC)
				if err {
					return true
				}
				l := in.NamedList(action.First())
				l_out := bytecode.List{}
				f_in := Interpreter{V: &Vars{
					Names: make(map[string]int),
				}}
				f_in.Copy(in)
				fn := in.NamedFunc(action.Second())
				for _, ptr := range l.Ids {
					a := in.GetAnyRef(ptr)
					if fn.Node != "" {
						f_in.Save(string(fn.Vars[0]), a)
						min_err := f_in.Run(fn.Node)
						if min_err {
							return true
						}
						if _, ok := f_in.V.Names["_return_"]; ok {
							ListAppend(&l_out, in, f_in.GetAny("_return_"))
						} else {
							in.Nothing("Nothing")
							nptr := &bytecode.MinPtr{uint64(in.GetSlot("Nothing").Index), in.Id}
							l_out.Ids = append(l_out.Ids, nptr)
						}
						f_in.Destroy()
						f_in = Interpreter{V: &Vars{
							Names: make(map[string]int),
						}}
						f_in.Copy(in)
					} else {
						f_in.Save("_item_", a)
						act2 := bytecode.Action{}
						act2.Source = action.Source
						act2.Target = "_return_"
						act2.Variables = []bytecode.Variable{("_item_")}
						act2.Type = fn.Name
						min_err := f_in.Run(act2.String())
						if min_err {
							return true
						}
						if _, ok := f_in.V.Names["_return_"]; ok {
							ListAppend(&l_out, in, f_in.GetAny("_return_"))
						} else {
							in.Nothing("Nothing")
							nptr := &bytecode.MinPtr{uint64(in.GetSlot("Nothing").Index), in.Id}
							l_out.Ids = append(l_out.Ids, nptr)
						}
					}
				}
				in.Save(action.Target, l_out)
				f_in.Destroy()
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
			case "html_set_inner":
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
				go_err := input.SetInnerHtml(in.NamedStr(action.First()), in.NamedStr(action.Second()))
				if go_err != nil {
					in.Error(action, go_err.Error(), "sys")
					return true
				}
			case "convert":
				err := in.CheckArgN(action, 2, 2)
				if err {
					return true
				}
				target_type := in.Type(action.Second())
				if target_type == in.Type(action.First()) {
					in.Save(action.Target, in.GetAny(string(action.Variables[0])))
				} else {
					switch target_type {
					case STR:
						switch in.Type(action.First()) {
						case INT:
							in.Save(action.Target, in.NamedInt(string(action.Variables[0])).String())
						case FLOAT:
							in.Save(action.Target, in.NamedFloat(string(action.Variables[0])).String())
						case BYTE:
							in.Save(action.Target, fmt.Sprintf("b.%d", in.NamedByte(string(action.Variables[0]))))
						case BOOL:
							in.Save(action.Target, fmt.Sprintf("%v", in.NamedBool(string(action.Variables[0]))))
						case LIST:
							l := in.NamedList(action.First())
							in.Save(action.Target, ListString(&l, in))
						case SPAN:
							s := in.NamedSpan(string(action.Variables[0]))
							switch s.Dtype {
							case BYTE:
								if in.NamedStr(action.Second()) == "x" {
									in.Save(action.Target, fmt.Sprintf("%x", in.V.Bytes[s.Start:s.Start+s.Length]))
								} else {
									in.Save(action.Target, string(in.V.Bytes[s.Start:s.Start+s.Length]))
								}
							default:
								in.Save(action.Target, in.StringSpan(s))
							}
						}
					case INT:
						switch in.Type(action.First()) {
						case STR:
							i := big.NewInt(0)
							i.SetString(in.NamedStr(action.First()), 10)
							in.Save(action.Target, i)
						case FLOAT:
							v, _ := in.NamedFloat(string(action.Variables[0])).Int(big.NewInt(0))
							in.Save(action.Target, v)
						case BYTE:
							v := in.NamedByte(string(action.Variables[0]))
							in.Save(action.Target, big.NewInt(int64(v)))
						}
					case FLOAT:
						switch in.Type(action.First()) {
						case INT:
							v, _ := in.NamedInt(string(action.Variables[0])).Float64()
							in.Save(action.Target, big.NewFloat(v))
						case BYTE:
							in.Save(action.Target, big.NewFloat(float64(in.NamedByte(string(action.Variables[0])))))
						}
					case BYTE:
						switch in.Type(action.First()) {
						case FLOAT:
							v, _ := in.NamedFloat(string(action.Variables[0])).Int64()
							in.Save(action.Target, byte(v))
						case INT:
							v := in.NamedInt(string(action.Variables[0])).Int64()
							in.Save(action.Target, byte(v))
						}
					case LIST:
						switch in.Type(action.First()) {
						case SPAN:
							l := bytecode.List{}
							s := in.NamedSpan(string(action.Variables[0]))
							for n := s.Start; n < s.Start+s.Length; n++ {
								var v any = big.NewInt(0)
								switch s.Dtype { // TODO: add all possible data types
								case INT:
									v = in.V.Ints[n]
								case BYTE:
									v = in.V.Bytes[n]
								}
								ListAppend(&l, in, v)
							}
							in.Save(action.Target, l)
						}
					case SPAN:
						switch in.Type(action.First()) {
						case STR:
							str := in.NamedStr(string(action.Variables[0]))
							data, go_err := hex.DecodeString(str)
							if go_err != nil {
								in.Error(action, go_err.Error(), "sys")
								return true
							}
							s := in.NewSpan(len(data), BYTE)
							for n := 0; n < len(data); n++ {
								in.V.Bytes[s.Start+uint64(n)] = data[n]
							}
							in.Save(action.Target, s)
						}
					}
				}
			case "value":
				err := in.CheckArgN(action, 1, 1) && in.CheckDtype(action, 0, ID)
				if err {
					in.Error(action, "error retrieving data from provided id", "id")
					return true
				}
				id := in.NamedId(action.First()) //in.GetAny(string(action.Variables[0])).(*bytecode.MinPtr)
				interp := in
				for interp.Id != id.Id {
					interp = interp.Parent
				}
				in.Save(action.Target, interp.GetAnyRef(id))
			case "read":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return true
				}
				b, berr := os.ReadFile(in.NamedStr(action.First()))
				if berr != nil {
					in.Error(action, berr.Error(), "file")
				}
				a := in.NewSpan(len(b), BYTE)
				for n, bb := range b {
					in.SpanSet(&a, n, bb)
				}
				in.Save(action.Target, a)
			case "write":
				if IsSafe {
					in.Error(action, "cannot write to files when in safe mode!", "permission")
					return true
				}
				err := in.CheckArgN(action, 2, 2)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return true
				}
				err = in.CheckDtype(action, 1, SPAN, STR)
				if err {
					return true
				}
				if in.Type(action.Second()) == SPAN {
					s := in.NamedSpan(string(action.Variables[1]))
					if s.Dtype == BYTE {
						oserr := os.WriteFile(in.NamedStr(string(action.Variables[0])), in.V.Bytes[s.Start:s.Start+s.Length], 0777)
						if oserr != nil {
							in.Error(action, oserr.Error(), "sys")
							return true
						}
					}
				} else {
					str := in.NamedStr(action.Second())
					oserr := os.WriteFile(in.NamedStr(action.First()), []byte(str), 0777)
					if oserr != nil {
						in.Error(action, oserr.Error(), "sys")
						return true
					}
				}
			case "mkdir":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return true
				}
				go_err := os.MkdirAll(in.NamedStr(action.First()), 0777)
				if go_err != nil {
					in.Error(action, go_err.Error(), "sys")
					return true
				}
			case "remove":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return true
				}
				go_err := os.Remove(in.NamedStr(action.First()))
				if go_err != nil {
					in.Error(action, go_err.Error(), "sys")
					return true
				}
			case "len":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, STR, LIST, SPAN)
				if err {
					return true
				}
				switch in.V.Slots[in.V.Names[string(action.Variables[0])]].Type {
				case STR:
					in.Save(action.Target, big.NewInt(int64(len([]rune(in.NamedStr(string(action.Variables[0])))))))
				case LIST:
					in.Save(action.Target, big.NewInt(int64(len(in.NamedList(string(action.Variables[0])).Ids))))
				case SPAN:
					s := in.NamedSpan(string(action.Variables[0]))
					in.Save(action.Target, big.NewInt(int64(s.Length)))
				}
			case "sleep":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, INT, FLOAT, BYTE)
				if err {
					return err
				}
				switch in.Type(action.First()) {
				case INT:
					time.Sleep(time.Duration(in.NamedInt(action.First()).Int64()) * 1000 * time.Millisecond)
				case FLOAT:
					f, _ := in.NamedFloat(action.First()).Float64()
					time.Sleep(time.Duration(int64(f*1000)) * time.Millisecond)
				case BYTE:
					time.Sleep(time.Duration(int64(in.NamedByte(action.First()))) * 1000 * time.Millisecond)
				}
			case "range":
				err := in.CheckArgN(action, 1, 3)
				if err {
					return err
				}
				i := in.NamedInt(string(action.Variables[0]))
				s := bytecode.Span{} //in.NewSpan(int(i.Int64()), INT)
				iterated := big.NewInt(0)
				step := big.NewInt(1)
				if len(action.Variables) > 1 {
					iterated.Set(in.NamedInt(action.First()))
					i.Set(in.NamedInt(action.Second()))
					if len(action.Variables) > 2 {
						step.Set(in.NamedInt(action.Third()))
						s = in.NewSpan(int(i.Int64()-iterated.Int64())/int(step.Int64()), INT)
					} else {
						s = in.NewSpan(int(i.Int64()-iterated.Int64()), INT)
					}
				} else {
					s = in.NewSpan(int(i.Int64()), INT)
				}
				counter := uint64(0)
				for iterated.Cmp(i) == -1 {
					in.SpanSet(&s, int(counter), iterated)
					iterated.Add(iterated, step)
					i2 := big.NewInt(0)
					i2.Set(iterated)
					iterated = i2
					counter++
				}
				in.Save(action.Target, s)
			case "span":
				err := in.CheckDtype(action, 0, LIST)
				if err {
					return err
				}
				l := in.NamedList(action.First())
				if len(l.Ids) == 0 {
					in.Error(action, "cannot create a span with length 0", "index")
					return true
				}
				s := in.NewSpan(len(l.Ids), in.TypeRef(l.Ids[0]))
				for n, item := range l.Ids {
					go_err := in.SpanSet(&s, n, in.GetAnyRef(item))
					if go_err != nil {
						in.Error(action, go_err.Error(), "index")
						return true
					}
				}
				in.Save(action.Target, s)
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
				o, t := in.EqualizeTypes(action.First(), action.Second())
				switch in.Type(o) {
				case INT:
					minimal = float64(in.NamedInt(o).Int64())
					maximal = float64(in.NamedInt(t).Int64())
				case FLOAT:
					minimal, _ = in.NamedFloat(o).Float64()
					maximal, _ = in.NamedFloat(t).Float64()
				case BYTE:
					minimal = float64(in.NamedByte(o))
					maximal = float64(in.NamedByte(t))
				}
				i := rand.Float64()*(maximal-minimal) + minimal
				in.Save(action.Target, big.NewFloat(i))
			case "sort":
				err := in.CheckArgN(action, 1, 2)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, LIST)
				if err {
					return err
				}
				// func start
				fn := bytecode.Function{}
				if len(action.Variables) > 1 {
					err = in.CheckDtype(action, 1, FUNC)
					if err {
						return err
					}
					fn = *in.NamedFunc(action.Second())
					if fn.Node != "" && len(fn.Vars) == 1 {
						mask := bytecode.List{}
						for ptr := 0; ptr < len(in.NamedList(action.First()).Ids); ptr++ {
							// user functions start
							f_in := Interpreter{V: &Vars{
								Names: make(map[string]int),
							}}
							f_in.Id = rand.Uint64()
							f_in.Copy(in)
							f_in.Save(string(fn.Vars[0]), in.GetAnyRef(in.NamedList(action.First()).Ids[ptr]))
							err := f_in.Run(fn.Node)
							in.ErrSource = f_in.ErrSource
							if err {
								return err
							}
							_, ok := f_in.V.Names["_return_"]
							if !ok {
								f_in.Nothing("_return_")
							}
							if f_in.V.Slots[f_in.V.Names["_return_"]].Type == LIST {
								l := in.CopyList("_return_", &f_in)
								//in.Save(action.Target, l)
								ListAppend(&mask, in, l)
							} else if f_in.V.Slots[f_in.V.Names["_return_"]].Type == PAIR {
								l := in.CopyPair("_return_", &f_in)
								//in.Save(action.Target, l)
								ListAppend(&mask, in, l)
							} else {
								//in.Save(action.Target, f_in.GetAny("_return_"))
								ListAppend(&mask, in, f_in.GetAny("_return_"))
							}
							f_in.Destroy()
							// user functions end
						}
						combined := bytecode.List{}
						for n := range mask.Ids {
							double := bytecode.List{}
							ListAppend(&double, in, in.GetAnyRef(mask.Ids[n]))
							ListAppend(&double, in, in.GetAnyRef(in.NamedList(action.First()).Ids[n]))
							ListAppend(&combined, in, double)
						}
						combined_sorted, go_err := in.SortList(combined)
						if go_err != nil {
							in.Error(action, go_err.Error(), "value")
							return true
						}
						combined_second := bytecode.List{}
						for n := range combined_sorted.Ids {
							two := in.GetAnyRef(combined_sorted.Ids[n]).(bytecode.List)
							ListAppend(&combined_second, in, in.GetAnyRef(two.Ids[1]))
						}
						in.Save(action.Target, combined_second)
					} else if fn.Node == "" {
						mask := bytecode.List{}
						for ptr := 0; ptr < len(in.NamedList(action.First()).Ids); ptr++ {
							// user functions start
							f_in := Interpreter{V: &Vars{
								Names: make(map[string]int),
							}}
							f_in.Id = rand.Uint64()
							f_in.Copy(in)
							f_in.Save("_item_", in.GetAnyRef(in.NamedList(action.First()).Ids[ptr]))
							action2 := bytecode.Action{}
							action2.Type = action.Second()
							action2.Variables = []bytecode.Variable{("_item_")}
							action2.Target = "_return_"
							action2.Source = action.Source
							err := f_in.Run(action2.String())
							in.ErrSource = f_in.ErrSource
							if err {
								return err
							}
							_, ok := f_in.V.Names["_return_"]
							if !ok {
								f_in.Nothing("_return_")
							}
							if f_in.V.Slots[f_in.V.Names["_return_"]].Type == LIST {
								l := in.CopyList("_return_", &f_in)
								//in.Save(action.Target, l)
								ListAppend(&mask, in, l)
							} else if f_in.V.Slots[f_in.V.Names["_return_"]].Type == PAIR {
								l := in.CopyPair("_return_", &f_in)
								//in.Save(action.Target, l)
								ListAppend(&mask, in, l)
							} else {
								//in.Save(action.Target, f_in.GetAny("_return_"))
								ListAppend(&mask, in, f_in.GetAny("_return_"))
							}
							f_in.Destroy()
							// user functions end
						}
						combined := bytecode.List{}
						for n := range mask.Ids {
							double := bytecode.List{}
							ListAppend(&double, in, in.GetAnyRef(mask.Ids[n]))
							ListAppend(&double, in, in.GetAnyRef(in.NamedList(action.First()).Ids[n]))
							ListAppend(&combined, in, double)
						}
						combined_sorted, go_err := in.SortList(combined)
						if go_err != nil {
							in.Error(action, go_err.Error(), "value")
							return true
						}
						combined_second := bytecode.List{}
						for n := range combined_sorted.Ids {
							two := in.GetAnyRef(combined_sorted.Ids[n]).(bytecode.List)
							ListAppend(&combined_second, in, in.GetAnyRef(two.Ids[1]))
						}
						in.Save(action.Target, combined_second)
					} else if fn.Node == "" {
						// this entire block is cope for the fact that built-in functions are not really the same as user ones
						node_name_sort := fmt.Sprintf("_runner_%x", rand.Int64())
						target_sort := fmt.Sprintf("_targ_%x", rand.Int64())
						arguments := []bytecode.Variable{("item")}
						in.Code[node_name_sort] = []bytecode.Action{{Target: target_sort, Type: fn.Name, Variables: arguments, Source: action.Source},
							{Type: "return", Variables: []bytecode.Variable{bytecode.Variable(target_sort)}, Source: action.Source}}
						mask := bytecode.List{}
						for ptr := 0; ptr < len(in.NamedList(action.First()).Ids); ptr++ {
							// user functions start
							f_in := Interpreter{V: &Vars{
								Names: make(map[string]int),
							}}
							f_in.Id = rand.Uint64()
							f_in.Copy(in)
							f_in.Save("item", in.GetAnyRef(in.NamedList(action.First()).Ids[ptr]))
							err := f_in.Run(node_name_sort)
							in.ErrSource = f_in.ErrSource
							if err {
								return err
							}
							_, ok := f_in.V.Names["_return_"]
							if !ok {
								f_in.Nothing("_return_")
							}
							if f_in.V.Slots[f_in.V.Names["_return_"]].Type == LIST {
								l := in.CopyList("_return_", &f_in)
								//in.Save(action.Target, l)
								ListAppend(&mask, in, l)
							} else if f_in.V.Slots[f_in.V.Names["_return_"]].Type == PAIR {
								l := in.CopyPair("_return_", &f_in)
								//in.Save(action.Target, l)
								ListAppend(&mask, in, l)
							} else {
								//in.Save(action.Target, f_in.GetAny("_return_"))
								ListAppend(&mask, in, f_in.GetAny("_return_"))
							}
							f_in.Destroy()
							// user functions end
						}
						delete(in.Code, node_name_sort)
						combined := bytecode.List{}
						for n := range mask.Ids {
							double := bytecode.List{}
							ListAppend(&double, in, in.GetAnyRef(mask.Ids[n]))
							ListAppend(&double, in, in.GetAnyRef(in.NamedList(action.First()).Ids[n]))
							ListAppend(&combined, in, double)
						}
						combined_sorted, go_err := in.SortList(combined)
						if go_err != nil {
							in.Error(action, go_err.Error(), "value")
							return true
						}
						combined_second := bytecode.List{}
						for n := range combined_sorted.Ids {
							two := in.GetAnyRef(combined_sorted.Ids[n]).(bytecode.List)
							ListAppend(&combined_second, in, in.GetAnyRef(two.Ids[1]))
						}
						in.Save(action.Target, combined_second)
					} else {
						in.Error(actions[focus], "Undeclared function!", "undeclared")
						return true
					}
				} else {
					sorted_l, sort_err := in.SortList(in.NamedList(action.First()))
					if sort_err != nil {
						in.Error(action, sort_err.Error(), "value")
						return true
					}
					in.Save(action.Target, sorted_l)
				}
				//func end
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
			case "exit":
				err := in.CheckArgN(action, 0, 1)
				if err {
					return err
				}
				if len(action.Variables) > 0 {
					err = in.CheckDtype(action, 0, INT)
					if err {
						return err
					}
					os.Exit(int(in.NamedInt(action.First()).Int64()))
				}
				os.Exit(0)
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
				case "arch":
					in.Save(actions[focus].Target, runtime.GOARCH)
				case "version":
					in.Save(actions[focus].Target, "4.3.4")
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
				case "file":
					in.Save(action.Target, ternary(in.File != nil, *in.File, ""))
				case "funcs":
					fs := bytecode.List{}
					interp := in
					for addr, slot := range interp.V.Slots {
						if slot.Type == FUNC {
							fs.Ids = append(fs.Ids, &bytecode.MinPtr{Addr: uint64(addr), Id: interp.Id})
						}
					}
					for interp.Parent != nil {
						interp = interp.Parent
						for addr, slot := range interp.V.Slots {
							if slot.Type == FUNC {
								fs.Ids = append(fs.Ids, &bytecode.MinPtr{Addr: uint64(addr), Id: interp.Id})
							}
						}
					}
					in.Save(action.Target, fs)
				case "vars":
					vs := bytecode.Pair{make(map[string]*bytecode.MinPtr)}
					interp := in
					for interp != nil {
						for name, i := range interp.V.Names {
							vs.Ids["str:"+name] = &bytecode.MinPtr{Addr: uint64(i), Id: interp.Id}
							// PairAppend(&vs, in, in.GetAnyRef(&bytecode.MinPtr{Id: interp.Id, Addr: uint64(i)}), name)
						}
						interp = interp.Parent
					}
					in.Save(action.Target, vs)
				}
			case "chdir":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return true
				}
				os.Chdir(in.NamedStr(string(action.Variables[0])))
			case "glob":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return true
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return true
				}
				files, go_err := filepath.Glob(in.NamedStr(action.First()))
				if go_err != nil {
					in.Error(action, go_err.Error(), "sys")
					return true
				}
				l := bytecode.List{}
				for _, file := range files {
					ListAppend(&l, in, file)
				}
				in.Save(action.Target, l)
			case "rget":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return true
				}
				if in.CheckDtype(action, 0, STR) {
					return true
				}
				url := in.NamedStr(string(action.Variables[0]))
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
				pnew.Ids = make(map[string]*bytecode.MinPtr)
				PairAppend(&pnew, in, big.NewInt(int64(resp.StatusCode)), "code")
				PairAppend(&pnew, in, string(body), "body")
				in.Save(actions[focus].Target, pnew)
				resp.Body.Close()
			case "jsonp":
				p := in.JsonPair([]byte(in.NamedStr(action.First())))
				in.Save(action.Target, p)
			case "rpost":
				err := in.CheckArgN(action, 2, 2)
				if err {
					return true
				}
				if in.CheckDtype(action, 0, STR) && in.CheckDtype(action, 1, PAIR) {
					return true
				}
				url := in.NamedStr(action.First())
				pair := in.NamedPair(action.Second())
				// jsonStr := PairString(&pair, in)
				jsonBytes, go_err := json.Marshal(in.PairToJson(pair))
				if go_err != nil {
					in.Error(action, go_err.Error(), "json")
					return true
				}
				resp, err2 := http.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
				if err2 != nil {
					in.Error(action, err2.Error(), "sys")
					return true
				}

				body, err3 := io.ReadAll(resp.Body)
				if err3 != nil {
					in.Error(action, err3.Error(), "sys")
					return true
				}
				pnew := bytecode.Pair{}
				pnew.Ids = make(map[string]*bytecode.MinPtr)
				PairAppend(&pnew, in, big.NewInt(int64(resp.StatusCode)), "code")
				if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
					PairAppend(&pnew, in, string(body), "body")
				} else {
					PairAppend(&pnew, in, string(body), "body")
				}
				in.Save(actions[focus].Target, pnew)
				resp.Body.Close()
			case "split":
				if err := in.CheckArgN(action, 2, 2); err {
					return err
				}
				if err := in.CheckDtype(action, 0, STR); err {
					return err
				}
				if err := in.CheckDtype(action, 1, STR); err {
					return err
				}
				arr := strings.Split(in.NamedStr(string(action.Variables[0])), in.NamedStr(string(action.Variables[1])))
				l := bytecode.List{}
				for _, element := range arr {
					ListAppend(&l, in, element)
				}
				in.Save(action.Target, l)
			case "join":
				// join list with separator -> !join ["hello", "world"], " "
				if err := in.CheckArgN(action, 2, 2); err {
					return err
				}
				if err := in.CheckDtype(action, 0, LIST); err {
					return err
				}
				if err := in.CheckDtype(action, 1, STR); err {
					return err
				}
				var to_join []string
				for _, ref := range in.NamedList(string(action.Variables[0])).Ids {
					v := in.GetAnyRef(ref)
					switch vtyped := v.(type) {
					case string:
						to_join = append(to_join, vtyped)
					default:
						in.Error(action, "cannot join list with non-str items within", "type")
						return true
					}
				}
				in.Save(action.Target, strings.Join(to_join, in.NamedStr(string(action.Variables[1]))))
			case "cti":
				if err := in.CheckArgN(action, 1, 1); err {
					return err
				}
				if err := in.CheckDtype(action, 0, STR); err {
					return err
				}
				str := in.NamedStr(action.First())
				if len(str) == 0 {
					in.Error(action, "zero length string in cti", "index")
					return true
				}
				i64 := int64([]rune(str)[0])
				in.Save(action.Target, big.NewInt(i64))
			case "itc":
				if err := in.CheckArgN(action, 1, 1); err {
					return err
				}
				if err := in.CheckDtype(action, 0, INT); err {
					return err
				}
				i64 := in.NamedInt(action.First()).Int64()
				if i64 < 0 {
					in.Error(action, "negative integer conversion to str", "index")
					return true
				}
				in.Save(action.Target, string([]rune{rune(i64)}))
			case "stats":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, STR)
				if err {
					return err
				}
				info, go_err := os.Stat(in.NamedStr(action.First()))
				if go_err != nil {
					in.Error(action, go_err.Error(), "sys")
					return true
				}
				p := bytecode.Pair{Ids: make(map[string]*bytecode.MinPtr)}
				PairAppend(&p, in, info.Name(), "name")
				PairAppend(&p, in, info.IsDir(), "is_dir")
				PairAppend(&p, in, big.NewInt(info.Size()), "size")
				PairAppend(&p, in, big.NewInt(info.ModTime().Unix()), "mod_time")
				PairAppend(&p, in, info.ModTime().UTC().Format("2006/01/02 15:04:05"), "mod_date")
				in.Save(action.Target, p)
			case "id":
				err := in.CheckArgN(action, 1, 2)
				if err {
					return err
				}
				if len(action.Variables) == 1 {
					interp := in
					_, ok := interp.V.Names[action.First()]
					for !ok {
						interp = interp.Parent
						_, ok = interp.V.Names[action.First()]
					}
					// fmt.Println("Pointer:", interp.V.Names[action.First()], interp.Id)
					in.Save(action.Target, &bytecode.MinPtr{uint64(interp.V.Names[action.First()]), interp.Id})
				} else {
					err = in.CheckDtype(action, 1, ID)
					if err {
						return err
					}
					interp := in
					ptr := in.NamedId(action.Second())
					for interp.Id != ptr.Id {
						interp = interp.Parent
					}
					interp.SaveRef(ptr, in.GetAny(action.First()))
					// newptr := interp.SaveRefNew(in.GetAny(action.First()))
					// interp.V.Slots[ptr.Addr] = interp.V.Slots[newptr.Addr]
					// TODO: check if removal of last Slot is needed
					// interp.SaveRef(ptr, in.GetAny(action.First()))
				}
			case "append":
				err := in.CheckArgN(action, 2, 2)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, LIST, SPAN)
				if err {
					return err
				}
				switch in.V.Slots[in.V.Names[string(action.Variables[0])]].Type {
				case LIST:
					l := in.NamedList(string(action.Variables[0]))
					ListAppend(&l, in, in.GetAny(string(action.Variables[1])))
					in.Save(action.Target, l)
				case SPAN:
					s := in.NamedSpan(string(action.Variables[0]))
					switch s.Dtype {
					case INT:
						err := in.CheckDtype(action, 1, INT)
						if err {
							return err
						}
						new_span := in.NewSpan(int(s.Length)+1, INT) // bytecode.Span{INT, uint64(new_start), new_length}
						for n := s.Start; n < s.Start+s.Length; n++ {
							relative_n := n - s.Start
							in.V.Ints[new_span.Start+relative_n].Set(in.V.Ints[n])
						}
						in.V.Ints[new_span.Start+new_span.Length-1].Set(in.NamedInt(string(action.Variables[1])))
						in.Save(action.Target, new_span)
						in.V.gcCycle = in.V.gcMax - 1 // make sure the GC actually works
						// in.GC()                       // this one is here to prevent memory overfill
					case FLOAT:
						err := in.CheckDtype(action, 1, FLOAT)
						if err {
							return err
						}
						new_span := in.NewSpan(int(s.Length)+1, FLOAT) // bytecode.Span{INT, uint64(new_start), new_length}
						for n := s.Start; n < s.Start+s.Length; n++ {
							relative_n := n - s.Start
							in.V.Floats[new_span.Start+relative_n].Set(in.V.Floats[n])
						}
						in.V.Floats[new_span.Start+new_span.Length-1].Set(in.NamedFloat(action.Second()))
						in.Save(action.Target, new_span)
						in.V.gcCycle = in.V.gcMax - 1
					case BYTE:
						err := in.CheckDtype(action, 1, BYTE)
						if err {
							return err
						}
						new_span := in.NewSpan(int(s.Length)+1, BYTE)
						for n := s.Start; n < s.Start+s.Length; n++ {
							relative_n := n - s.Start
							in.V.Bytes[new_span.Start+relative_n] = in.V.Bytes[n]
						}
						in.V.Bytes[new_span.Start+new_span.Length-1] = in.NamedByte(action.Second())
						in.Save(action.Target, new_span)
						in.V.gcCycle = in.V.gcMax - 1
					}
				}
			case "has":
				err := in.CheckArgN(action, 2, 2)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, STR, LIST, SPAN)
				if err {
					return err
				}
				switch in.V.Slots[in.V.Names[string(action.Variables[0])]].Type {
				case STR:
					err = in.CheckDtype(action, 1, STR)
					if err {
						return err
					}
					in.Save(action.Target, strings.Contains(in.NamedStr(string(action.Variables[0])), in.NamedStr(string(action.Variables[1]))))
				case LIST:
					in.Save(action.Target, false)
					l := in.NamedList(string(action.Variables[0]))
					for n := range len(l.Ids) {
						in.Save("_cmp_0", in.GetAnyRef(l.Ids[n]))
						in.Save("_cmp_1", in.GetAny(string(action.Variables[1])))
						o, t := in.EqualizeTypes("_cmp_0", "_cmp_1")
						if equals := in.CompareName(o, t); equals {
							in.Save(action.Target, true)
							break
						}
					}
				case SPAN:
					in.Save(action.Target, false)
					s := in.NamedSpan(string(action.Variables[0]))
					for n := range s.Length {
						switch s.Dtype {
						case INT:
							// TODO: add more types
							in.Save("_cmp_0", in.V.Ints[s.Start+n])
						}
						in.Save("_cmp_1", in.GetAny(string(action.Variables[1])))
						o, t := in.EqualizeTypes("_cmp_0", "_cmp_1")
						if equals := in.CompareName(o, t); equals {
							in.Save(action.Target, true)
							break
						}
					}
				}
			case "where":
				err := in.CheckArgN(action, 2, 2)
				if err {
					return err
				}
				err = in.CheckDtype(action, 0, STR, LIST, SPAN)
				if err {
					return err
				}
				switch in.V.Slots[in.V.Names[string(action.Variables[0])]].Type {
				case STR:
					err = in.CheckDtype(action, 1, STR)
					if err {
						return err
					}
					in.Save(action.Target, big.NewInt(int64(strings.Index(in.NamedStr(string(action.Variables[0])), in.NamedStr(string(action.Variables[1]))))))
				case LIST:
					in.Save(action.Target, big.NewInt(-1))
					l := in.NamedList(string(action.Variables[0]))
					for n := range len(l.Ids) {
						in.Save("_cmp_0", in.GetAnyRef(l.Ids[n]))
						in.Save("_cmp_1", in.GetAny(string(action.Variables[1])))
						o, t := in.EqualizeTypes("_cmp_0", "_cmp_1")
						if equals := in.CompareName(o, t); equals {
							in.Save(action.Target, big.NewInt(int64(n)))
							break
						}
					}
				case SPAN:
					in.Save(action.Target, false)
					s := in.NamedSpan(string(action.Variables[0]))
					for n := range s.Length {
						switch s.Dtype {
						case INT:
							// TODO: add more types
							in.Save("_cmp_0", in.V.Ints[s.Start+n])
						}
						in.Save("_cmp_1", in.GetAny(string(action.Variables[1])))
						o, t := in.EqualizeTypes("_cmp_0", "_cmp_1")
						if equals := in.CompareName(o, t); equals {
							in.Save(action.Target, big.NewInt(int64(n)))
							break
						}
					}
				}
			case "check_type":
				dtypes_map := map[byte]string{NOTH: "noth", INT: "int", FLOAT: "float", BYTE: "byte", STR: "str", FUNC: "func", SPAN: "span", ID: "id", LIST: "list", BOOL: "bool", PAIR: "pair", ARR: "arr"}
				type_byte := in.Type(action.First())
				type_string := in.NamedStr(action.Second()) // TODO TYPECHECK
				if dtypes_map[type_byte] != type_string {
					in.Error(action, fmt.Sprintf("type mismatch: %s instead of %s", type_string, dtypes_map[type_byte]), "type")
					return true
				}
			case "type":
				err := in.CheckArgN(action, 1, 1)
				if err {
					return err
				}
				in.Save(action.Target, map[byte]string{NOTH: "noth", INT: "int", FLOAT: "float", BYTE: "byte", STR: "str", FUNC: "func", SPAN: "span", ID: "id", LIST: "list", BOOL: "bool", PAIR: "pair", ARR: "arr"}[in.Type(action.First())])
			default:
				if fn.Node != "" {
					// user functions start
					f_in := Interpreter{V: &Vars{
						Names: make(map[string]int),
					}}
					f_in.Id = rand.Uint64()
					f_in.Copy(in)
					for n, fn_arg := range fn.Vars {
						if n == len(fn.Vars)-1 && len(action.Variables) > len(fn.Vars) {
							last := bytecode.List{}
							for _, lastlet := range action.Variables[len(fn.Vars)-1:] {
								ListAppend(&last, &f_in, in.GetAny(string(lastlet)))
							}
							f_in.Save(string(fn_arg), last)
							continue
						}
						fn_arg_str := string(fn_arg)
						if in.Type(string(action.Variables[n])) == PAIR {
							p := f_in.CopyPair(string(action.Variables[n]), in)
							f_in.Save(fn_arg_str, p)
						} else if in.Type(string(action.Variables[n])) == LIST {
							l := f_in.CopyList(string(action.Variables[n]), in)
							f_in.Save(fn_arg_str, l)
						} else if in.Type(string(action.Variables[n])) == ID {
							id := in.NamedId(string(action.Variables[n]))
							f_in.Save(fn_arg_str, id)
						} else {
							f_in.Save(fn_arg_str, in.GetAny(string(action.Variables[n])))
						}
					}
					err := f_in.Run(fn.Node)
					in.ErrSource = f_in.ErrSource
					if err {
						return err
					}
					_, ok := f_in.V.Names["_return_"]
					if !ok {
						f_in.Nothing("_return_")
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
					return true
				}
			}
		}
		focus++
	}
	return false
}

func (in *Interpreter) DeepAssign(receiver any, item any, inds []any) error {
	switch rec := receiver.(type) {
	case *bytecode.List:
		if len(inds) == 1 {
			switch ind := inds[0].(type) {
			case *big.Int:
				i := int(ind.Int64())
				if i < 0 {
					i += len(rec.Ids)
				}
				if i < 0 || i >= len(rec.Ids) {
					return fmt.Errorf("impossible index: %d for list of length %d", i, len(rec.Ids))
				}
				rec.Ids[i] = in.GetRef(item)
			}
		} else {
			switch ind := inds[0].(type) {
			case *big.Int:
				i := int(ind.Int64())
				if i < 0 {
					i += len(rec.Ids)
				}
				if i < 0 || i >= len(rec.Ids) {
					return fmt.Errorf("impossible index: %d for list of length %d", i, len(rec.Ids))
				}
				if in.V.Slots[rec.Ids[i].Addr].Type == LIST {
					sublist := in.GetAnyRef(rec.Ids[i]).(bytecode.List)
					return in.DeepAssign(&sublist, item, inds[1:])
				}
				subpair := in.GetAnyRef(rec.Ids[i]).(bytecode.Pair)
				return in.DeepAssign(&subpair, item, inds[1:])
			}
		}
	case *bytecode.Pair:
		if len(inds) == 1 {
			mainkey := PairKey(in, inds[0])
			if _, ok := rec.Ids[mainkey]; ok {
				rec.Ids[mainkey] = in.GetRef(item)
			} else {
				PairAppend(rec, in, item, inds[0])
			}
		} else {
			mainkey := PairKey(in, inds[0])
			if _, ok := rec.Ids[mainkey]; ok {
				rec.Ids[mainkey] = in.GetRef(item)
			} else {
				PairAppend(rec, in, item, inds[0])
			}
			if in.V.Slots[rec.Ids[mainkey].Addr].Type == LIST {
				sublist := in.GetAnyRef(rec.Ids[mainkey]).(bytecode.List)
				return in.DeepAssign(&sublist, item, inds[1:])
			}
			subpair := in.GetAnyRef(rec.Ids[mainkey]).(bytecode.Pair)
			return in.DeepAssign(&subpair, item, inds[1:])
		}
	default:
		types := map[byte]string{NOTH: "noth", INT: "int", FLOAT: "float", BYTE: "byte", STR: "str", FUNC: "func", SPAN: "span", ID: "id", LIST: "list", BOOL: "bool", PAIR: "pair", ARR: "arr"}
		return fmt.Errorf("unsupported assignment target: %s", types[TypeToByte(rec)])
	}
	return nil
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
func (in *Interpreter) Nothing(name string) {
	interp := in
	for interp.Parent != nil {
		if slot_id, ok := interp.V.Names["Nothing"]; !ok {
			interp = interp.Parent
		} else if interp.V.Slots[slot_id].Type == NOTH {
			return
		}
	}
	in.Save(name, byte(0))
	in.V.Slots[in.V.Names[name]].Type = NOTH
}

func (in *Interpreter) Type(name string) byte {
	if _, ok := in.V.Names[name]; !ok {
		return in.Parent.Type(name)
	}
	return in.V.Slots[in.V.Names[name]].Type
}

func (in *Interpreter) IndexList(l bytecode.List, ind_any any) (any, error) {
	switch ind := ind_any.(type) {
	case *big.Int:
		ind_int := int(ind.Int64())
		if ind_int < 0 {
			ind_int += len(l.Ids)
		}
		if ind_int < 0 {
			return nil, fmt.Errorf("impossible index")
		}
		return in.GetAnyRef(l.Ids[ind_int]), nil
	case bytecode.List:
		nl := bytecode.List{}
		for _, ptr := range ind.Ids {
			sub_ind := in.GetAnyRef(ptr)
			sub_item, err := in.IndexList(l, sub_ind)
			if err != nil {
				return nil, err
			}
			ListAppend(&nl, in, sub_item)
		}
		return nl, nil
	default:
		return nil, fmt.Errorf("unsupported index type")
	}
}

func (in *Interpreter) TypeRef(ref *bytecode.MinPtr) byte {
	if ref.Id != in.Id {
		return in.Parent.TypeRef(ref)
	}
	return in.V.Slots[ref.Addr].Type
}

func (in *Interpreter) CompareName(n0, n1 string) bool {
	if n0 == n1 || in.V.Names[n0] == in.V.Names[n1] {
		return true
	}
	if in.Type(n0) != in.Type(n1) {
		return false
	}
	return in.Compare(in.GetAny(n0), in.GetAny(n1))
}

func (in *Interpreter) Compare(v0, v1 any) bool {
	// TODO: add pair comparison
	switch v0t := v0.(type) {
	case *big.Int:
		v1t := v1.(*big.Int)
		return v0t.Cmp(v1t) == 0
	case *big.Float:
		v1t := v1.(*big.Float)
		return v0t.Cmp(v1t) == 0
	case string:
		return v0t == v1.(string)
	case bool:
		return v0t == v1.(bool)
	case byte:
		return v0t == v1.(byte)
	case uint64:
		return v0t == v1.(uint64)
	case *bytecode.Function:
		return v0t == v1.(*bytecode.Function)
	case bytecode.List:
		v1t := v1.(bytecode.List)
		if len(v0t.Ids) != len(v1t.Ids) {
			return false
		}
		for n := range v0t.Ids {
			interp := in
			for v0t.Ids[n].Id != interp.Id {
				interp = interp.Parent
			}
			if interp.V.Slots[v0t.Ids[n].Addr].Type != interp.V.Slots[v1t.Ids[n].Addr].Type {
				return false
			}
			if !in.Compare(in.GetAnyRef(v0t.Ids[n]), in.GetAnyRef(v1t.Ids[n])) {
				return false
			}
			return true
		}
	case bytecode.Span:
		v1t := v1.(bytecode.Span)
		if v0t.Length != v1t.Length || v0t.Dtype != v1t.Dtype {
			return false
		}
		if v0t.Start == v1t.Start {
			return true
		}
		//TODO: expand types
		switch v0t.Dtype {
		case INT:
			for n := range v0t.Length {
				if !in.Compare(in.V.Ints[v0t.Start+n], in.V.Ints[v1t.Start+n]) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func (in *Interpreter) Copy(og *Interpreter) {
	in.IgnoreErr = og.IgnoreErr
	in.Code = og.Code
	in.File = og.File
	in.Parent = og
	/*
		for key := range og.V.Names {
			in.Save(key, og.GetAny(key))
		}
	*/
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
			newPair := bytecode.Pair{Ids: make(map[string]*bytecode.MinPtr)}
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
	l := og.NamedList(vname) //og.GetAny(vname).(bytecode.List)
	lnew := bytecode.List{}
	for _, id := range l.Ids {
		a := og.GetAnyRef(id)
		if og.TypeRef(id) == LIST {
			a = in.CopyListRef(id, og)
		} else if og.TypeRef(id) == PAIR {
			a = in.CopyPairRef(id, og)
		}
		newid := in.SaveRefNew(a)
		lnew.Ids = append(lnew.Ids, newid)
	}
	return lnew
}

func (in *Interpreter) CopyListRef(vname *bytecode.MinPtr, og *Interpreter) bytecode.List {
	l := og.GetAnyRef(vname).(bytecode.List)
	lnew := bytecode.List{}
	for _, id := range l.Ids {
		a := og.GetAnyRef(id)
		if og.TypeRef(id) == LIST {
			a = in.CopyListRef(id, og)
		} else if og.TypeRef(id) == PAIR {
			a = in.CopyPairRef(id, og)
		}
		newid := in.SaveRefNew(a)
		lnew.Ids = append(lnew.Ids, newid)
	}
	return lnew
}

func (in *Interpreter) CopyPair(vname string, og *Interpreter) bytecode.Pair {
	l := og.GetAny(vname).(bytecode.Pair)
	lnew := bytecode.Pair{}
	lnew.Ids = make(map[string]*bytecode.MinPtr)
	for key, id := range l.Ids {
		a := og.GetAnyRef(id)
		if og.TypeRef(id) == LIST {
			a = in.CopyListRef(id, og)
		} else if og.TypeRef(id) == PAIR {
			a = in.CopyPairRef(id, og)
		}
		newid := in.SaveRefNew(a)
		lnew.Ids[key] = newid
	}
	return lnew
}

func (in *Interpreter) CopyPairRef(vname *bytecode.MinPtr, og *Interpreter) bytecode.Pair {
	l := og.GetAnyRef(vname).(bytecode.Pair)
	lnew := bytecode.Pair{}
	lnew.Ids = make(map[string]*bytecode.MinPtr)
	for key, id := range l.Ids {
		a := og.GetAnyRef(id)
		if og.TypeRef(id) == LIST {
			a = in.CopyListRef(id, og)
		} else if og.TypeRef(id) == PAIR {
			a = in.CopyPairRef(id, og)
		}
		newid := in.SaveRefNew(a)
		lnew.Ids[key] = newid
	}
	return lnew
}

func (in *Interpreter) Destroy() {
	//for key := range in.V.Names {
	//	in.RemoveName(key)
	//}

	in.GCE()
	in = &Interpreter{}
}

func (in *Interpreter) CopyListDeep(l bytecode.List, og *Interpreter) bytecode.List {
	nl := bytecode.List{}
	for _, ref := range l.Ids {
		a := og.GetAnyRef(ref)
		if og.V.Slots[ref.Addr].Type == LIST {
			a = in.CopyListDeep(a.(bytecode.List), og)
		}
		ptr := in.SaveRefNew(a)
		nl.Ids = append(nl.Ids, ptr)
	}
	return nl
}

// isDirectory checks if the given path refers to a directory.
func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		// If the file/directory does not exist, os.Stat returns an error.
		// You might want to handle os.IsNotExist(err) specifically if
		// you need to distinguish between "not found" and other errors.
		return false, err
	}
	return fileInfo.IsDir(), nil
}

func (in *Interpreter) HeaderFunc(signature string) {
	// pass a proper signature and get a complete Minimum function
	signature = strings.ReplaceAll(signature, " ", "")
	splitted := strings.Split(signature, ":")
	fn_name := splitted[0]
	rest := splitted[1]
	rest = rest[1 : len(rest)-1]
	allowed_inputs := strings.Split(rest, ")(")
	for _, allowed_input := range allowed_inputs {
		input_length := len(strings.Split(allowed_input, ","))
		fn_name_args := "_fn_" + fn_name + strconv.Itoa(input_length)
		header := "func " + fn_name_args + " "
		in_vars := []bytecode.Variable{}
		for n := range input_length {
			header += "i" + strconv.Itoa(n) + ", "
			in_vars = append(in_vars, bytecode.Variable("i"+strconv.Itoa(n)))
		}
		header += ":"
		body := []string{}
		for ind, dtype := range strings.Split(allowed_input, ",") {
			dtypes := strings.Split(dtype, "|")
			for n := range dtypes {
				dtypes[n] = "\"" + dtypes[n] + "\""
			}
			if strings.Contains(dtype, "any") {
				continue
			}
			statement := fmt.Sprintf(" !check_type i%d, %s", ind, strings.Join(dtypes, ", "))
			body = append(body, statement)
		}
		fn_full := strings.Join(append([]string{header}, body...), "\n")
		in.Compile(fn_full, ".")
		last_node := fmt.Sprintf("_node_%d", bytecode.NodeN-1)
		in.Save(fn_name_args, &bytecode.Function{fn_name, last_node, in_vars, last_node})
	}
}

func ShowSource(source string) {
	length := 0
	lines := strings.Split(source, "\n")
	for _, line := range lines {
		if len([]rune(line)) > length {
			length = len([]rune(line))
		}
	}
	pad := ""
	for range length {
		pad += "_"
	}
	lines = append([]string{pad}, append(lines, pad)...)
	fmt.Println(strings.Join(lines, "\n"))
}

func (in *Interpreter) PairToJson(p bytecode.Pair) map[string]any {
	m := make(map[string]any)
	for key, ptr := range p.Ids {
		splitted := strings.SplitN(key, ":", 2)
		key_type := splitted[0]
		key_real := splitted[1]
		switch key_type {
		case "str":
			v := in.GetAnyRef(ptr)
			switch v_typed := v.(type) {
			case *big.Int:
				i := v_typed.Int64()
				m[key_real] = i
			case *big.Float:
				f, _ := v_typed.Float64()
				m[key_real] = f
			case bytecode.Pair:
				m[key_real] = in.PairToJson(v_typed)
			case bytecode.List:
				m[key_real] = in.ListToJson(v_typed)
			default:
				m[key_real] = v_typed
			}
		case "int":
			v := in.GetAnyRef(ptr)
			switch v_typed := v.(type) {
			case *big.Int:
				i := v_typed.Int64()
				m[key_real] = i
			case *big.Float:
				f, _ := v_typed.Float64()
				m[key_real] = f
			case bytecode.Pair:
				m[key_real] = in.PairToJson(v_typed)
			case bytecode.List:
				m[key_real] = in.ListToJson(v_typed)
			default:
				m[key_real] = v_typed
			}
		}
	}
	return m
}

func (in *Interpreter) ListToJson(l bytecode.List) []any {
	lany := []any{}
	for _, ptr := range l.Ids {
		v := in.GetAnyRef(ptr)
		switch v_typed := v.(type) {
		case *big.Int:
			i := v_typed.Int64()
			lany = append(lany, i)
		case *big.Float:
			f, _ := v_typed.Float64()
			lany = append(lany, f)
		case bytecode.Pair:
			lany = append(lany, in.PairToJson(v_typed))
		case bytecode.List:
			lany = append(lany, in.ListToJson(v_typed))
		default:
			lany = append(lany, v_typed)
		}
	}
	return lany
}

type RunRequest struct {
	Variables []string `json:"variables"`
	Code      string   `json:"code"`
}

var ServerInterpreter *Interpreter

func ServerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	compile := r.URL.Query().Get("compile")
	//bodyBytes, _ := io.ReadAll(r.Body)
	//println(string(bodyBytes))
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if ServerInterpreter == nil {
		ServerInterpreter = &Interpreter{
			V:    &Vars{Names: make(map[string]int)},
			Id:   rand.Uint64(),
			Code: make(map[string][]bytecode.Action),
		}
	}

	result := ServerInterpreter.RunJson(req, compile == "")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (in *Interpreter) RunJson(req RunRequest, forget bool) map[string]any {
	// Track existing nodes so we can delete temporary ones later
	var originalNodes []string
	if forget {
		for node := range in.Code {
			originalNodes = append(originalNodes, node)
		}
	}

	// Compile the code
	in.Compile(req.Code, "json")
	lastNode := fmt.Sprintf("_node_%d", bytecode.NodeN-1)

	// Remove GC action
	if len(in.Code[lastNode]) > 1 {
		in.Code[lastNode] = in.Code[lastNode][:len(in.Code[lastNode])-1]
	}

	// Run the code
	in.Run(lastNode)

	// Collect results
	result := make(map[string]any)
	for _, name := range req.Variables {
		val := in.GetAny(name)

		switch in.Type(name) {
		case INT:
			val = val.(*big.Int).Int64()
		case FLOAT:
			val, _ = val.(*big.Float).Float64()
		case LIST:
			val = in.ListToJson(val.(bytecode.List))
		case PAIR:
			val = in.PairToJson(val.(bytecode.Pair))
		}

		result[name] = val
	}

	if !forget {
		return result
	}
	// Remove temporary compiled nodes
	for node := range in.Code {
		if !bytecode.Has(originalNodes, node) {
			delete(in.Code, node)
		}
	}

	return result
}

var IsSafe bool

func (in *Interpreter) JsonPair(obj []byte) bytecode.Pair {
	target := make(map[string]any)
	json.Unmarshal(obj, &target)
	p := bytecode.Pair{Ids: make(map[string]*bytecode.MinPtr)}
	for key, valany := range target {
		switch val := valany.(type) {
		case int:
			PairAppend(&p, in, big.NewInt(int64(val)), key)
		case float64:
			PairAppend(&p, in, big.NewFloat(val), key)
		case string:
			PairAppend(&p, in, val, key)
		case []any:
			b, _ := json.Marshal(val)
			PairAppend(&p, in, in.JsonList(b), key)
		case map[string]any:
			b, _ := json.Marshal(val)
			PairAppend(&p, in, in.JsonPair(b), key)
		default:
			PairAppend(&p, in, val, key)

		}
	}
	return p
}

func (in *Interpreter) JsonList(obj []byte) bytecode.List {
	var val []any
	json.Unmarshal(obj, &val)
	l := bytecode.List{}
	for _, a := range val {
		switch val_item := a.(type) {
		case int:
			ListAppend(&l, in, big.NewInt(int64(val_item)))
		case float64:
			ListAppend(&l, in, big.NewFloat(val_item))
		case string:
			ListAppend(&l, in, val)
		case []any:
			b, _ := json.Marshal(val_item)
			ListAppend(&l, in, in.JsonList(b))
		case map[string]any:
			b, _ := json.Marshal(val_item)
			ListAppend(&l, in, in.JsonPair(b))
		default:
			ListAppend(&l, in, val_item)
		}
	}
	return l
}
