package bytecode

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"unicode/utf8"
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

func Has[T comparable](series []T, what T) bool {
	for _, item := range series {
		if item == what {
			return true
		}
	}
	return false
}

func Index[T comparable](series []T, what T) int {
	for n, item := range series {
		if item == what {
			return n
		}
	}
	return -1
}

func Unlink[T any](series []T) []T {
	empty := []T{}
	return append(empty, series...)
}

type MinPtr struct {
	Addr uint64
	Id   uint64
}

func (mp *MinPtr) String() string {
	return fmt.Sprintf("%x@%x", mp.Addr, mp.Id)
}

type Token struct {
	Type  string
	Value string
}

func (t *Token) String() string {
	if t.Value == "" {
		return fmt.Sprintf("[%s]", t.Type)
	} else {
		return fmt.Sprintf("[%s: %s]", t.Type, t.Value)
	}
}

func ToksPrint(tokens []Token) {
	for _, t := range tokens {
		fmt.Print(t.String())
	}
	fmt.Println()
}

func ToksMatch(tokens []Token, reg *regexp.Regexp) bool {
	identifier := ""
	for _, tok := range tokens {
		identifier += fmt.Sprintf("<%s>", tok.Type)
	}
	return reg.MatchString(identifier)
}

func keys_sorted(keys []string) bool {
	for n := 1; n < len(keys); n++ {
		if len(keys[n-1]) < len(keys[n]) {
			return false
		}
	}
	return true
}

func sort_keys(mkeys map[string]string) []string {
	keys := []string{}
	for key := range mkeys {
		keys = append(keys, key)
	}
	for !keys_sorted(keys) {
		for n := 1; n < len(keys); n++ {
			if len(keys[n-1]) < len(keys[n]) {
				keys[n], keys[n-1] = keys[n-1], keys[n]
				break
			}
		}
	}
	return keys
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

func remove_comment(str string) string {
	splitted := strings.Split(str, "\n")
	for n := 0; n < len(splitted); n++ {
		splitted[n] = strings.Split(splitted[n], "#")[0]
	}
	return strings.Join(splitted, "\n")
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
func DotConst(tokens []Token) []Token {
	reg_const := regexp.MustCompile(`^(-?[0-9]+)$`)
	points := []int{}
	for n := 0; n < len(tokens); n++ {
		if tokens[n].Type == "DOT" {
			points = append(points, n)
		}
	}
	reversed := []int{}
	for n := len(points) - 1; n > -1; n-- {
		reversed = append(reversed, points[n])
	}
	for _, ind := range reversed {
		left, right := ind-1, ind+1
		if tokens[left].Type == "CONST" && reg_const.MatchString(tokens[left].Value) && tokens[right].Type == "CONST" && reg_const.MatchString(tokens[right].Value) {
			tokens2 := tokens[:left]
			tokens2 = append(tokens2, Token{"CONST", tokens[left].Value + "." + tokens[right].Value})
			tokens2 = append(tokens2, tokens[right+1:]...)
			tokens = Unlink(tokens2)
		} else if tokens[left].Type == "WORD" && tokens[left].Value == "b" && tokens[right].Type == "CONST" && reg_const.MatchString(tokens[right].Value) {
			tokens2 := tokens[:left]
			tokens2 = append(tokens2, Token{"CONST", tokens[left].Value + "." + tokens[right].Value})
			tokens2 = append(tokens2, tokens[right+1:]...)
			tokens = Unlink(tokens2)
		}
	}
	return tokens
}

func oop(tokens []Token) []Token {
	if Has(tokens, Token{"DOT", ""}) {
		ind := 0
		for n := range len(tokens) {
			if tokens[n].Type == "DOT" && tokens[n-1].Type == "WORD" {
				ind = n
				break
			}
		}
		tokens[ind] = Token{"SUB", ""}
		tokens[ind+1] = Token{"CONST", "\"" + tokens[ind+1].Value + "\""}
	}
	return Unlink(tokens)
	reg_const := regexp.MustCompile(`^(-?[0-9]+)$`)
	points := []int{}
	for n := 0; n < len(tokens); n++ {
		if tokens[n].Type == "DOT" {
			points = append(points, n)
		}
	}
	reversed := []int{}
	for n := len(points) - 1; n > -1; n-- {
		reversed = append(reversed, points[n])
	}
	for _, ind := range reversed {
		left, right := ind-1, ind+1
		if tokens[left].Type == "CONST" && reg_const.MatchString(tokens[left].Value) && tokens[right].Type == "CONST" && reg_const.MatchString(tokens[right].Value) {
			tokens2 := tokens[:left]
			tokens2 = append(tokens2, Token{"CONST", tokens[left].Value + "." + tokens[right].Value})
			tokens2 = append(tokens2, tokens[right+1:]...)
			tokens = Unlink(tokens2)
		} else if tokens[left].Type == "WORD" && tokens[left].Value == "b" && tokens[right].Type == "CONST" && reg_const.MatchString(tokens[right].Value) {
			tokens2 := tokens[:left]
			tokens2 = append(tokens2, Token{"CONST", tokens[left].Value + "." + tokens[right].Value})
			tokens2 = append(tokens2, tokens[right+1:]...)
			tokens = Unlink(tokens2)
		}
	}
	return tokens
}

func Tokenize(sourcestr string) []Token {
	reg_const := regexp.MustCompile(`^(true|false|(-?[0-9]+\.[0-9]+)|(-?[0-9]+)|(b\.[0-9]+)|".*")$`)
	constants := map[string]string{"$": "DOLL", ",": "COMM", ".": "DOT", "'": "SUB", " or ": "OR", " and ": "AND", "not ": "NOT", ":": "COL", "}": "C_CUR", "{": "O_CUR", "]": "C_BR", "[": "O_BR", ")": "C_PAR", "(": "O_PAR", "!": "ACT", "++": "PP", "--": "MM", "+": "PLUS", "->": "R_ARR", "-": "MINUS", "*": "MUL", "//": "DDIV", "/": "DIV", "^": "POW", "%": "MOD", "<": "LESS", ">": "GREAT", "==": "ISEQ", "!=": "NISEQ", "<-": "L_ARR", "&=": "PEQ", "=": "EQ"}
	focus := 0
	output := []Token{}
	buffer := ""
	source := []string{}
	for _, char := range sourcestr {
		source = append(source, string(char))
	}
	for focus < len(source) {
		changed := false
		for _, key := range sort_keys(constants) {
			if strings.HasPrefix(strings.Join(source[focus:], ""), key) {
				if buffer != "" {
					buffer = strings.TrimSpace(buffer)
					if reg_const.MatchString(buffer) {
						output = append(output, Token{"CONST", buffer})
					} else {
						output = append(output, Token{"WORD", buffer})
					}
					buffer = ""
				}
				output = append(output, Token{constants[key], ""})
				focus += len(key)
				changed = true
				break
			}
		}
		if !changed {
			if source[focus] == " " {
				if buffer != "" {
					buffer = strings.TrimSpace(buffer)
					if reg_const.MatchString(buffer) {
						output = append(output, Token{"CONST", buffer})
					} else {
						output = append(output, Token{"WORD", buffer})
					}
					buffer = ""
				}
				focus++
				continue
			}
			buffer += string(source[focus])
			focus++
		}
	}
	if buffer != "" {
		buffer = strings.TrimSpace(buffer)
		if reg_const.MatchString(buffer) {
			output = append(output, Token{"CONST", buffer})
		} else {
			output = append(output, Token{"WORD", buffer})
		}
		buffer = ""
	}
	output = unary(output)
	output = DotConst(output)
	output = oop(output)
	return output
}

func unary(tokens []Token) []Token {
	ops := []string{"MINUS", "NOT"}
	for n := 1; n < len(tokens); n++ {
		if !Has([]string{"WORD", "CONST", "C_PAR"}, tokens[n-1].Type) && Has(ops, tokens[n].Type) {
			t := Unlink(tokens[:n])
			t = append(t, Token{"O_PAR", ""})
			t = append(t, Token{"CONST", "0"})
			t = append(t, tokens[n:n+2]...)
			t = append(t, Token{"C_PAR", ""})
			t = append(t, tokens[n+2:]...)
			tokens = Unlink(t)
			n = 0
			continue
		}
	}
	return tokens
}

func Test() {
	c := GetCode("if true:\n  a = !len my_list, 11\nelse:\n  !(obj.fun) \"xd\"\n$\"ls\"\n!print !len my_list")
	for key := range c {
		acts := c[key]
		fmt.Println(key + ":")
		for _, action := range acts {
			vs := []string{}
			for _, variable := range action.Variables {
				vs = append(vs, string(variable))
			}
			fmt.Printf("  %s <-[%s]- %s\n", action.Target, action.Type, strings.Join(vs, ", "))
		}
	}
}

func PrintActs(c map[string][]Action) {
	for key := range c {
		acts := c[key]
		fmt.Println(key + ":")
		for _, action := range acts {
			vs := []string{}
			for _, variable := range action.Variables {
				vs = append(vs, string(variable))
			}
			fmt.Printf("  %s <-[%s]- %s\n", action.Target, action.Type, strings.Join(vs, ", "))
		}
	}
}

type Variable string

type Action struct {
	Target    string
	Type      string
	Variables []Variable
	Source    *SourceLine
}

func (a *Action) First() string {
	return string(a.Variables[0])
}

func (a *Action) Second() string {
	return string(a.Variables[1])
}

func (a *Action) Third() string {
	return string(a.Variables[2])
}

var TempN int

func HasAct(tokens []Token) int {
	ind := -1
	for n := len(tokens) - 1; n >= 0; n-- {
		if tokens[n].Type == "ACT" {
			ind = n
		}
	}
	return ind
}

func HasActLeft(tokens []Token) int {
	ind := -1
	for n := len(tokens) - 1; n >= 0; n-- {
		if tokens[n].Type == "ACT" {
			ind = n
			break
		}
	}
	return ind
}

func HasOps(tokens []Token, ops []string) bool {
	for _, token := range tokens {
		if Has(ops, token.Type) {
			return true
		}
	}
	return false
}

func GetOp(tokens []Token) int {
	ops := [][]string{{"NOT"}, {"SUB", "DOT"}, {"POW"}, {"MUL", "DIV", "DDIV", "MOD"}, {"PLUS", "MINUS"}, {"ISEQ", "NISEQ", "LESS", "GREAT"}, {"OR", "AND"}}
	for _, level := range ops {
		for n, token := range tokens {
			for _, op := range level {
				if op == token.Type {
					return n
				}
			}
		}
	}
	return -1
}

func TempName() string {
	name := fmt.Sprintf("_temp_%d", TempN)
	TempN++
	return name
}

type SourceLine struct {
	Source string
	N      int
}

func CommaArgs(tokens []Token) [][]Token {
	var args [][]Token
	var buffer []Token
	level := 0
	// "O_CUR", "]": "C_BR", "[": "O_BR", ")": "C_PAR"
	for _, tok := range tokens {
		if tok.Type == "COMM" && level == 0 {
			args = append(args, Unlink(buffer))
			buffer = []Token{}
		} else if tok.Type == "O_PAR" || tok.Type == "O_CUR" || tok.Type == "O_BR" {
			level++
			buffer = append(buffer, tok)
		} else if tok.Type == "C_PAR" || tok.Type == "C_CUR" || tok.Type == "C_BR" {
			level--
			buffer = append(buffer, tok)
		} else {
			buffer = append(buffer, tok)
		}
	}
	if len(buffer) > 0 {
		args = append(args, Unlink(buffer))
		buffer = []Token{}
	}
	return args
}

func HasParen(tokens []Token) bool {
	for n := 0; n < len(tokens); n++ {
		if tokens[n].Type == "O_PAR" || tokens[n].Type == "C_PAR" {
			return true
		}
	}
	return false
}

func HasCur(tokens []Token) bool {
	for n := 0; n < len(tokens); n++ {
		if tokens[n].Type == "O_CUR" || tokens[n].Type == "C_CUR" {
			return true
		}
	}
	return false
}

func HasParenWhere(tokens []Token) (int, int) {
	for n := len(tokens) - 1; n >= 0; n-- {
		if tokens[n].Type == "C_PAR" {
			end := n
			for m := end; m >= 0; m-- {
				if tokens[m].Type == "O_PAR" {
					start := m
					return start, end
				}
			}
		}
	}
	return -1, len(tokens)
}

func HasParenWhereOuter(tokens []Token) (int, int) {
	start := 0
	for tokens[start].Type != "O_PAR" {
		start++
	}
	end := start + 1
	level := 0
	for end < len(tokens) {
		if tokens[end].Type == "C_PAR" && level == 0 {
			break
		}
		if tokens[end].Type == "O_PAR" {
			level++
		} else if tokens[end].Type == "C_PAR" {
			level--
		}
		end++
	}
	return start, end
}

func HasCurWhereOuter(tokens []Token) (int, int) {
	start := 0
	for tokens[start].Type != "O_CUR" {
		start++
	}
	end := start + 1
	level := 0
	for end < len(tokens) {
		if tokens[end].Type == "C_CUR" && level == 0 {
			break
		}
		if tokens[end].Type == "O_CUR" {
			level++
		} else if tokens[end].Type == "C_CUR" {
			level--
		}
		end++
	}
	return start, end
}

func ParenArgs(tokens []Token, actions []Action, sl *SourceLine) ([]Token, []Action) {
	start := HasAct(tokens)
	if tokens[start+1].Type == "O_PAR" {
		level := 0
		for n := start; n < len(tokens); n++ {
			if tokens[n].Type == "O_PAR" {
				level++
			} else if tokens[n].Type == "C_PAR" {
				if level == 0 {
					args := CommaArgs(tokens[n+1:])
					vs := []Variable{}
					for _, arg := range args {
						actionslet := GetActs(arg, sl)
						t := Variable(actionslet[len(actionslet)-1].Target)
						vs = append(vs, t)
						actions = append(actions, actionslet...)
					}
					actlet := GetActs(tokens[start+2:n], sl)
					targ := TempName()
					actions = append(actions, Action{targ, actlet[len(actlet)-1].Target, vs, sl})
					tokens = []Token{{"WORD", targ}}
					return tokens, actions
				}
				level--
			}
		}
	}
	return []Token{}, []Action{}
}

func HasList(tokens []Token) bool {
	for n := 0; n < len(tokens); n++ {
		if tokens[n].Type == "O_BR" {
			return true
		}
	}
	return false
}

func HasListWhere(tokens []Token) (int, int) {
	for n := 0; n < len(tokens); n++ {
		if tokens[n].Type == "O_BR" {
			level := 0
			for m := n; m < len(tokens); m++ {
				switch {
				case tokens[m].Type == "O_BR":
					level++
				case tokens[m].Type == "C_BR":
					level--
				}
				if level == 0 && tokens[m].Type == "C_BR" {
					return n, m
				}
			}
		}
	}
	return -1, -1
}

func GetTargetAuto(tokens []Token, actions *[]Action, sl *SourceLine) string {
	actlet := GetActs(tokens, sl)
	var targ string
	if len(actlet) < 1 {
		name := tokens[0].Value
		if tokens[0].Type == "CONST" {
			name = TempName()
			actlet = append(actlet, Action{name, "const", []Variable{Variable(tokens[0].Value)}, sl})
		}
		targ = name
	} else {
		targ = actlet[len(actlet)-1].Target
	}
	*actions = append(*actions, actlet...)
	return targ
}

func ModifierModifier(tokens []Token, ops []string) []Token {
	// a += 3 => a = a + 3
	eq_id := -1
	for n, tok := range tokens {
		if tok.Type == "EQ" || tok.Type == "PEQ" {
			eq_id = n
			break
		}
	}
	if eq_id > -1 && HasOps(tokens[eq_id-1:eq_id], ops) {
		modifier := tokens[eq_id-1]
		targ := Unlink(tokens[:eq_id-1])
		equals := tokens[eq_id]
		tail := Unlink(tokens[eq_id+1:])
		tokens_new := Unlink(tokens[:eq_id-1])
		tokens_new = append(tokens_new, equals)
		tokens_new = append(tokens_new, targ...)
		tokens_new = append(tokens_new, modifier)
		tokens_new = append(tokens_new, tail...)
		tokens = Unlink(tokens_new)
	}
	return tokens
}

func GetActs(tokens []Token, sl *SourceLine) []Action {
	var actions []Action
	ops := []string{"DOT", "SUB", "AND", "OR", "NOT", "PLUS", "MINUS", "MUL", "DIV", "DDIV", "MOD", "POW", "ISEQ", "NISEQ", "LESS", "GREAT"}
	action_map := map[string]string{"DOT": ".", "SUB": "'", "AND": "and", "OR": "or", "NOT": "not", "ISEQ": "==", "NISEQ": "!=", "LESS": "<", "GREAT": ">", "PLUS": "+", "MINUS": "-", "MUL": "*", "DIV": "/", "DDIV": "//", "MOD": "%", "POW": "^"}
	// TempN = 0 // might be unnecessary
	tokens = ModifierModifier(tokens, ops)

	// finding targets start
	var targets []string
	pointer := false
	eq_id := -1
	for n, tok := range tokens {
		if tok.Type == "EQ" || tok.Type == "PEQ" {
			if tok.Type == "PEQ" {
				pointer = true
			}
			eq_id = n
			break
		}
	}
	var targets_tok [][]Token
	if eq_id > -1 {
		targets_tok = CommaArgs(tokens[:eq_id])
		for _, targ_tok := range targets_tok {
			if len(targ_tok) == 1 { // if it is a single var name
				targets = append(targets, targ_tok[0].Value)
			} else {
				// TODO: make less hacky
				targets = append(targets, targ_tok[0].Value)
			}
		}
		tokens = Unlink(tokens[eq_id+1:])
	}
	// finding targets end
	done := false
	for !done {
		switch {
		case strings.HasPrefix(strings.TrimSpace(sl.Source), "$") || len(tokens) > 0 && Has(tokens, Token{"DOLL", ""}):
			actions = append(actions, Action{TempName(), "$", []Variable{}, sl})
			if len(targets) > 0 {
				actions[len(actions)-1].Type = "$$"
				actions[len(actions)-1].Target = targets[0]
			}
			tokens = []Token{}
			return actions
			/*
				actlet := GetActs(tokens[1:], sl)
				var t string
				if len(actlet) > 0 {
					t = actlet[len(actlet)-1].Target
				} else {
					if tokens[1].Type == "WORD" {
						t = tokens[1].Value
					} else {
						v := Variable(tokens[1].Value)
						temp := TempName()
						actions = append(actions, Action{temp, "const", []Variable{v}, sl})
						t = temp
					}
				}
				actions = append(actions, actlet...)
				actions = append(actions, Action{TempName(), "$", []Variable{Variable(t)}, sl})
				tokens = []Token{{"WORD", t}}
			*/
		case len(tokens) > 1 && tokens[0].Type == "WORD" && tokens[0].Value == "repeat" && tokens[len(tokens)-1].Type == "LINK" && tokens[len(tokens)-2].Type == "COL":
			actlet := GetActs(tokens[1:len(tokens)-2], sl)
			var t string
			if len(actlet) > 0 {
				t = actlet[len(actlet)-1].Target
			} else {
				if tokens[1].Type == "WORD" {
					t = tokens[1].Value
				} else {
					v := Variable(tokens[1].Value)
					temp := TempName()
					actions = append(actions, Action{temp, "const", []Variable{v}, sl})
					t = temp
				}
			}
			actions = append(actions, actlet...)
			actions = append(actions, Action{tokens[len(tokens)-1].Value, "repeat", []Variable{Variable(t)}, sl})
			tokens = []Token{{"WORD", t}}
		case len(tokens) > 1 && tokens[0].Type == "WORD" && tokens[0].Value == "error" && tokens[len(tokens)-1].Type == "LINK" && tokens[len(tokens)-2].Type == "COL":
			args := CommaArgs(tokens[1 : len(tokens)-2])
			target := tokens[len(tokens)-1].Value
			vs := []Variable{}
			// TODO: add deep assignment support
			switch len(args) {
			case 1:
				vs = append(vs, Variable(args[0][0].Value))
			case 2:
				vs = append(vs, Variable(args[0][0].Value))
				vs = append(vs, Variable(args[1][0].Value))
			}
			actions = append(actions, Action{target, "error", vs, sl})
			tokens = []Token{{"WORD", target}}
		case len(tokens) > 1 && tokens[0].Type == "WORD" && tokens[0].Value == "if" && tokens[len(tokens)-1].Type == "LINK" && tokens[len(tokens)-2].Type == "COL":
			actlet := GetActs(tokens[1:len(tokens)-2], sl)
			var t string
			if len(actlet) > 0 {
				t = actlet[len(actlet)-1].Target
			} else {
				if tokens[1].Type == "WORD" {
					t = tokens[1].Value
				} else {
					v := Variable(tokens[1].Value)
					temp := TempName()
					actions = append(actions, Action{temp, "const", []Variable{v}, sl})
					t = temp
				}
			}
			actions = append(actions, actlet...)
			actions = append(actions, Action{tokens[len(tokens)-1].Value, "if", []Variable{Variable(t)}, sl})
			// actions = append(actions, Action{"", "endif", []Variable{}, sl})
			tokens = []Token{{"WORD", t}}
		case len(tokens) > 1 && tokens[0].Type == "WORD" && tokens[0].Value == "while" && tokens[len(tokens)-1].Type == "LINK" && tokens[len(tokens)-2].Type == "COL":
			actions = append(actions, Action{tokens[len(tokens)-1].Value, "while_start", []Variable{}, sl})
			actlet := GetActs(tokens[1:len(tokens)-2], sl)
			var t string
			if len(actlet) > 0 {
				t = actlet[len(actlet)-1].Target
			} else {
				if tokens[1].Type == "WORD" {
					t = tokens[1].Value
				} else {
					v := Variable(tokens[1].Value)
					temp := TempName()
					actions = append(actions, Action{temp, "const", []Variable{v}, sl})
					t = temp
				}
			}
			actions = append(actions, actlet...)
			actions = append(actions, Action{tokens[len(tokens)-1].Value, "while", []Variable{Variable(t)}, sl})
			// actions = append(actions, Action{"", "endif", []Variable{}, sl})
			tokens = []Token{{"WORD", t}}
		case len(tokens) > 1 && tokens[0].Type == "WORD" && tokens[0].Value == "for" && tokens[len(tokens)-1].Type == "LINK" && tokens[len(tokens)-2].Type == "COL":
			args_flat := tokens[1 : len(tokens)-2]
			args := CommaArgs(args_flat)
			vs := []Variable{}
			for _, arg := range args {
				ind := Index(arg, Token{"R_ARR", ""})
				left, right := arg[:ind], arg[ind+1:]
				actlet := GetActs(left, sl)
				var t string
				if len(actlet) == 0 {
					name := left[0].Value
					if left[0].Type == "CONST" {
						name = TempName()
						actions = append(actlet, Action{name, "const", []Variable{Variable(left[0].Value)}, sl})
					}
					//vs = append(vs, Variable(name))
					t = name
				} else {
					actions = append(actions, actlet...)
					t = actlet[len(actlet)-1].Target
				}
				vs = append(vs, Variable(t))
				vs = append(vs, Variable(right[0].Value))
			}
			act := Action{tokens[len(tokens)-1].Value, "for", vs, sl}
			actions = append(actions, act)
			tokens = []Token{} // SUS, might cause errors due to length 0
		case len(tokens) > 1 && tokens[0].Type == "WORD" && tokens[0].Value == "pool" && tokens[len(tokens)-1].Type == "LINK" && tokens[len(tokens)-2].Type == "COL":
			right_arrows := []int{}
			for n, token := range tokens {
				if token.Type == "R_ARR" {
					right_arrows = append(right_arrows, n)
				}
			}
			left_arrows := []int{}
			for n, token := range tokens {
				if token.Type == "L_ARR" {
					left_arrows = append(left_arrows, n)
				}
			}
			vs := []Variable{}
			for _, ra := range right_arrows {
				vs = append(vs, Variable(tokens[ra-1].Value))
				vs = append(vs, Variable(tokens[ra+1].Value))
			}
			vs = append(vs, Variable("Nothing"))
			for _, la := range left_arrows {
				vs = append(vs, Variable(tokens[la-1].Value))
				vs = append(vs, Variable(tokens[la+1].Value))
			}
			actions = append(actions, Action{Target: tokens[len(tokens)-1].Value, Variables: vs, Type: "pool", Source: sl})
			tokens = []Token{} // SUS, might cause errors due to length 0
		case HasParen(tokens):
			start, end := HasParenWhereOuter(tokens)
			expr := tokens[start+1 : end]
			actlet := GetActs(expr, sl)
			var targ string
			if len(actlet) < 1 {
				name := expr[0].Value
				if expr[0].Type == "CONST" {
					name = TempName()
					actlet = append(actlet, Action{name, "const", []Variable{Variable(expr[0].Value)}, sl})
				}
				targ = name
			} else {
				targ = actlet[len(actlet)-1].Target
			}
			actions = append(actions, actlet...)
			newtok := tokens[:start]
			newtok = append(newtok, Token{"WORD", targ})
			newtok = append(newtok, tokens[end+1:]...)
			tokens = Unlink(newtok)
		case len(tokens) > 1 && HasCur(tokens):
			start, end := HasCurWhereOuter(tokens)
			args := CommaArgs(tokens[start+1 : end])
			var targets []Variable
			for _, arg := range args {
				sep := 0
				for arg[sep].Type != "COL" {
					sep++
				}
				key_tok := arg[:sep]
				val_tok := arg[sep+1:]
				t0, t1 := GetTargetAuto(key_tok, &actions, sl), GetTargetAuto(val_tok, &actions, sl)
				targets = append(targets, Variable(t0))
				targets = append(targets, Variable(t1))
			}
			t := TempName()
			a := Action{Target: t, Variables: targets, Type: "pair", Source: sl}
			actions = append(actions, a)
			tail := tokens[end+1:]
			head := tokens[:start]
			head = append(head, Token{"WORD", t})
			head = append(head, tail...)
			tokens = Unlink(head)
		case len(tokens) > 1 && HasList(tokens):
			is_array := false
			atype := byte(NOTH)
			arr_types := map[byte]string{0: "noth", 1: "int", 2: "float", 3: "str", 4: "arr", 5: "list", 6: "pair", 7: "bool", 8: "byte", 9: "func", 10: "id"}
			start, end := HasListWhere(tokens)
			if start-1 > -1 && tokens[start-1].Type == "DOT" {
				t := tokens[start-2].Value
				for key, val := range arr_types {
					if val == t {
						atype = key
						is_array = true
					}
				}
			}
			args := CommaArgs(tokens[start+1 : end])
			vs := []Variable{}
			for _, arg := range args {
				actionslet := GetActs(arg, sl)
				var t Variable
				if len(actionslet) > 0 {
					t = Variable(actionslet[len(actionslet)-1].Target)
				} else {
					// TODO: add constant support
					name := arg[0].Value
					if arg[0].Type == "CONST" {
						name = TempName()
						actionslet = append(actionslet, Action{name, "const", []Variable{Variable(arg[0].Value)}, sl})
					}
					t = Variable(name)
				}
				vs = append(vs, t)
				actions = append(actions, actionslet...)
			}
			targ := TempName()
			if !is_array {
				actions = append(actions, Action{targ, "list", vs, sl})
				tail := Unlink(tokens[end+1:])
				tokens = append(tokens[:start], []Token{{"WORD", targ}}...)
				tokens = append(tokens, tail...)
			} else {
				type_name := TempName()
				actions = append(actions, Action{type_name, "const", []Variable{Variable(fmt.Sprintf("b.%d", atype))}, sl})
				actions = append(actions, Action{targ, "array", append([]Variable{Variable(type_name)}, vs...), sl})
				tail := Unlink(tokens[end+1:])
				tokens = append(tokens[:start-2], []Token{{"WORD", targ}}...)
				tokens = append(tokens, tail...)
			}
		case len(tokens) > 1 && tokens[0].Type == "WORD" && tokens[0].Value == "func" && tokens[len(tokens)-1].Type == "LINK" && tokens[len(tokens)-2].Type == "COL":
			func_name := tokens[1].Value
			args := CommaArgs(tokens[2 : len(tokens)-2])
			func_args := []Variable{Variable(func_name)}
			for _, arg := range args {
				func_args = append(func_args, Variable(arg[0].Value))
			}
			actions = append(actions, Action{tokens[len(tokens)-1].Value, "func", func_args, sl})
			tokens = []Token{{"WORD", tokens[len(tokens)-1].Value}}
		case len(tokens) > 1 && tokens[0].Type == "WORD" && tokens[0].Value == "switch" && tokens[len(tokens)-1].Type == "LINK" && tokens[len(tokens)-2].Type == "COL":
			actlet := GetActs(tokens[1:len(tokens)-2], sl)
			var t string
			if len(actlet) > 0 {
				t = actlet[len(actlet)-1].Target
			} else {
				if len(tokens) == 3 { // if argless switch
					v := Variable("true")
					temp := TempName()
					actions = append(actions, Action{temp, "const", []Variable{v}, sl})
					t = temp
				} else if tokens[1].Type == "WORD" {
					t = tokens[1].Value
				} else {
					v := Variable(tokens[1].Value)
					temp := TempName()
					actions = append(actions, Action{temp, "const", []Variable{v}, sl})
					t = temp
				}
			}
			actions = append(actions, actlet...)
			actions = append(actions, Action{tokens[len(tokens)-1].Value, "switch", []Variable{Variable(t)}, sl})
			tokens = []Token{{"WORD", t}}
		case len(tokens) > 1 && tokens[0].Type == "WORD" && tokens[0].Value == "case" && tokens[len(tokens)-1].Type == "LINK" && tokens[len(tokens)-2].Type == "COL":
			actlet := GetActs(tokens[1:len(tokens)-2], sl)
			var t string
			if len(actlet) > 0 {
				t = actlet[len(actlet)-1].Target
			} else {
				if len(tokens) == 3 { // if argless switch
					v := Variable("true")
					temp := TempName()
					actions = append(actions, Action{temp, "const", []Variable{v}, sl})
					t = temp
				} else if tokens[1].Type == "WORD" {
					t = tokens[1].Value
				} else {
					v := Variable(tokens[1].Value)
					temp := TempName()
					actions = append(actions, Action{temp, "const", []Variable{v}, sl})
					t = temp
				}
			}
			actions = append(actions, actlet...)
			actions = append(actions, Action{tokens[len(tokens)-1].Value, "case", []Variable{Variable(t)}, sl})
			tokens = []Token{{"WORD", t}}
		case len(tokens) > 1 && tokens[0].Type == "WORD" && tokens[0].Value == "else" && tokens[len(tokens)-1].Type == "LINK" && tokens[len(tokens)-2].Type == "COL":
			actions = append(actions, Action{tokens[len(tokens)-1].Value, "else", []Variable{}, sl})
			tokens = []Token{}
		case len(tokens) > 0 && tokens[0].Type == "WORD" && tokens[0].Value == "return":
			args := CommaArgs(tokens[1:])
			vs := []Variable{}
			for _, arg := range args {
				actionslet := GetActs(arg, sl)
				var t Variable
				if len(actionslet) > 0 {
					t = Variable(actionslet[len(actionslet)-1].Target)
				} else {
					// TODO: add constant support
					name := arg[0].Value
					if arg[0].Type == "CONST" {
						name = TempName()
						actionslet = append(actionslet, Action{name, "const", []Variable{Variable(arg[0].Value)}, sl})
					}
					t = Variable(name)
				}
				vs = append(vs, t)
				actions = append(actions, actionslet...)
			}
			targ := TempName()
			actions = append(actions, Action{targ, "return", vs, sl})
			tokens = []Token{{"WORD", targ}} //append(tokens[:ind], []Token{{"WORD", targ}}...)
		case HasAct(tokens) > -1:
			ind := HasActLeft(tokens)
			action := tokens[ind+1]
			args := CommaArgs(tokens[ind+2:])
			vs := []Variable{}
			for _, arg := range args {
				actionslet := GetActs(arg, sl)
				var t Variable
				if len(actionslet) > 0 {
					t = Variable(actionslet[len(actionslet)-1].Target)
				} else {
					// TODO: add constant support
					name := arg[0].Value
					if arg[0].Type == "CONST" {
						name = TempName()
						actionslet = append(actionslet, Action{name, "const", []Variable{Variable(arg[0].Value)}, sl})
					}
					t = Variable(name)
				}
				vs = append(vs, t)
				actions = append(actions, actionslet...)
			}
			targ := TempName()
			actions = append(actions, Action{targ, action.Value, vs, sl})
			tokens = append(tokens[:ind], []Token{{"WORD", targ}}...)
		case len(tokens) == 2 && tokens[0].Type == "WORD" && (tokens[1].Type == "PP" || tokens[1].Type == "MM"):
			t := tokens[0].Value
			act := Action{t, ternary(tokens[1].Type == "PP", "++", "--"), []Variable{Variable(tokens[0].Value)}, sl}
			actions = append(actions, act)
			tokens = []Token{{"WORD", t}}
		case HasOps(tokens, ops):
			ind := GetOp(tokens)
			var v0, v1 Variable
			if tokens[ind-1].Type == "WORD" {
				v0 = Variable(tokens[ind-1].Value)
			} else {
				vt := TempName()
				actions = append(actions, Action{vt, "const", []Variable{Variable(tokens[ind-1].Value)}, sl})
				v0 = Variable(vt)
			}
			if tokens[ind+1].Type == "WORD" {
				v1 = Variable(tokens[ind+1].Value)
			} else {
				vt := TempName()
				actions = append(actions, Action{vt, "const", []Variable{Variable(tokens[ind+1].Value)}, sl})
				v1 = Variable(vt)
			}
			name := TempName()
			actions = append(actions, Action{name, action_map[tokens[ind].Type], []Variable{v0, v1}, sl}) // TODO: actually add variables
			tail := tokens[ind+2:]
			tokens = append(tokens[:ind-1], Token{"WORD", name})
			tokens = append(tokens, tail...)
			tokens = Unlink(tokens)
		default:
			done = true
			break
		}
	}

	if len(targets) > 0 {
		// deep assignment start
		deep := false
		for _, targ := range targets_tok {
			if Has(targ, Token{"SUB", ""}) {
				nest := [][]Token{}
				for Has(targ, Token{"SUB", ""}) {
					for i := 0; i < len(targ); i++ {
						if targ[i].Type == "SUB" {
							nest = append(nest, targ[:i])
							targ = Unlink(targ[i+1:])
							break
						}
					}
				}
				nest = append(nest, Unlink(targ))
				item := GetTargetAuto(tokens[0:1], &actions, sl)
				ind_names := []Variable{Variable(nest[0][0].Value), Variable(item)}
				for n, expr := range nest {
					if n == 0 {
						continue
					}
					// actlet := GetActs(expr, sl)
					// targa := actlet[len(actlet)-1].Target
					targa := GetTargetAuto(expr, &actions, sl)
					ind_names = append(ind_names, Variable(targa))
					// actions = append(actions, actlet...)
				}
				action := Action{"", "sub", ind_names, sl}
				actions = append(actions, action)
				deep = true
			}
			// TODO: fix this horrible piece of shit
			if false && Has(targ, Token{"SUB", ""}) {
				deep = true
				/*
					actlet := GetActs(targ, sl)
					// t := TempName()
					auto_target := tokens[0].Value // GetTargetAuto(tokens, &actions, sl)
					if tokens[0].Type == "CONST" {
						tname := TempName()
						actions = append(actions, Action{tname, "const", []Variable{Variable(tokens[0].Value)}, sl})
						auto_target = tname
					}
					t := TempName()
					act := Action{t, "id", []Variable{Variable(auto_target)}, sl}
					act2 := Action{TempName(), "id", []Variable{Variable(t)}, sl}
				*/
				actlet := GetActs(targ, sl)
				// modify start
				for n := range len(actlet) {
					if actlet[n].Type == "'" {
						actlet[n].Type = "''"
					}
				}
				// modify end
				auto_target := tokens[0].Value // GetTargetAuto(tokens, &actions, sl)
				if tokens[0].Type == "CONST" {
					tname := TempName()
					actions = append(actions, Action{tname, "const", []Variable{Variable(tokens[0].Value)}, sl})
					auto_target = tname
				}
				action := Action{"", "deep", []Variable{Variable(actlet[len(actlet)-1].Target), Variable(auto_target)}, sl}
				actions = append(actions, actlet...)
				actions = append(actions, action)
			}
		}
		if deep {
			return actions
		}
		// deep assignment end
		if len(targets) == 1 {
			if pointer {
				if len(actions) > 0 {
					actions = append(actions, Action{targets[0], "&=", []Variable{Variable(actions[len(actions)-1].Target)}, sl})
				} else {
					if tokens[0].Type == "WORD" {
						actions = append(actions, Action{targets[0], "&=", []Variable{Variable(tokens[0].Value)}, sl})
					} else {
						tname := TempName()
						actions = append(actions, Action{tname, "const", []Variable{Variable(tokens[0].Value)}, sl})
						actions = append(actions, Action{targets[0], "&=", []Variable{Variable(tname)}, sl})
					}
				}
			} else {
				if len(actions) > 0 {
					actions = append(actions, Action{targets[0], "=", []Variable{Variable(actions[len(actions)-1].Target)}, sl})
				} else {
					if tokens[0].Type == "WORD" {
						actions = append(actions, Action{targets[0], "=", []Variable{Variable(tokens[0].Value)}, sl})
					} else {
						tname := TempName()
						actions = append(actions, Action{tname, "const", []Variable{Variable(tokens[0].Value)}, sl})
						actions = append(actions, Action{targets[0], "=", []Variable{Variable(tname)}, sl})
					}
				}
			}
		} else if len(targets) > 1 {
			var iterable Variable
			if len(actions) > 0 {
				iterable = Variable(actions[len(actions)-1].Target)
			} else {
				if tokens[0].Type == "WORD" {
					iterable = Variable(tokens[0].Value)
				} else {
					tname := TempName()
					actions = append(actions, Action{tname, "const", []Variable{Variable(tokens[0].Value)}, sl})
					iterable = Variable(tname)
				}
			}
			for n := range len(targets) {
				second := fmt.Sprintf("%d", n)
				sname := TempName()
				actions = append(actions, Action{sname, "const", []Variable{Variable(second)}, sl})
				actions = append(actions, Action{targets[n], "'", []Variable{iterable, Variable(sname)}, sl})
			}
		}
	}
	return actions
}

type CodePart struct {
	Line        string
	LineOG      string
	Indentation int
	N           int
	TargetNode  string
}

func GetInd(line string) int {
	for n := 0; n < len(line); n++ {
		if line[n] != ' ' {
			return n
		}
	}
	return 0
}

func fill_strings(source string, m map[string]string) string {
	splitted := strings.Split(source, "\"")
	lines := []string{splitted[0]}
	for n := 1; n < len(splitted); n += 2 {
		lines = append(lines, m[splitted[n]])
		lines = append(lines, splitted[n+1])
	}
	return strings.Join(lines, "\"")
}

func trim_all(strs []string) []string {
	for n := 0; n < len(strs); n++ {
		strs[n] = strings.TrimSpace(strs[n])
	}
	return strs
}

var NodeN uint64

func GetCode(source string) map[string][]Action {
	source = strings.ReplaceAll(source, "\r\n", "\n")
	compiled := make(map[string][]Action)
	parts := []CodePart{}
	source_no_strings, string_map := remove_strings(source)
	// comment removal start
	source_no_strings = remove_comment(source_no_strings)
	// comment removal end
	countables := make(map[string]int)
	countables = map[string]int{"[": 0, "\"": 0, "(": 0, "]": 0, ")": 0, "{": 0, "}": 0}
	buffer := []string{}
	series := strings.Split(source_no_strings, "\n")
	for ln, line := range series {
		if strings.TrimSpace(line) == "" { // experimental empty line remover
			continue
		}
		for key := range countables {
			countables[key] += strings.Count(line, key)
		}
		if countables["{"] > countables["}"] || countables["["] > countables["]"] || countables["("] > countables[")"] || countables["\""]%2 == 1 {
			buffer = append(buffer, line)
		} else {
			buffer = append(buffer, line)
			part := CodePart{Line: strings.Join(trim_all(Unlink(buffer)), "\n"), LineOG: fill_strings(strings.Join(trim_all(Unlink(buffer)), "\n"), string_map), Indentation: GetInd(strings.Join(buffer, "\n")), N: ln - (len(buffer) - 1)}
			// part := CodePart{strings.Join(trim_all(buffer), "\n"), fill_strings(strings.Join(trim_all(buffer), "\n"), string_map), GetInd(strings.Join(buffer, "\n")), ln - (len(buffer) - 1), ""} //Line{fill_strings(strings.Join(buffer, "\n"), string_map), strings.Join(trim_all(buffer), ""), ln}
			parts = append(parts, part)
			buffer = []string{}
		}
	}
	nodes := make(map[string][]CodePart)
	maximal_ind := 0
	for focus := 0; focus < len(parts); focus++ {
		if parts[focus].Indentation > maximal_ind {
			maximal_ind = parts[focus].Indentation
		}
	}
	for maximal_ind > 0 {
		for start := 0; start < len(parts)-1; start++ {
			if parts[start+1].Indentation > parts[start].Indentation && parts[start+1].Indentation == maximal_ind {
				body := start + 1
				cp := []CodePart{}
				for body < len(parts) && parts[body].Indentation == maximal_ind {
					cp = append(cp, parts[body])
					body++
				}
				nname := fmt.Sprintf("_node_%d", NodeN)
				NodeN++
				nodes[nname] = cp
				parts[start].TargetNode = nname
				parts = Unlink(append(parts[:start+1], parts[body:]...))
				break
			}
		}
		maximal_ind = 0
		for focus := 0; focus < len(parts); focus++ {
			if parts[focus].Indentation > maximal_ind {
				maximal_ind = parts[focus].Indentation
			}
		}
	}
	nname := fmt.Sprintf("_node_%d", NodeN)
	NodeN++
	nodes[nname] = Unlink(parts)
	for key := range nodes {
		node_acts := []Action{}
		for _, line := range nodes[key] {
			toks := Tokenize(line.Line)
			if line.TargetNode != "" {
				toks = append(toks, Token{"LINK", line.TargetNode})
			}
			sl := SourceLine{line.LineOG, line.N}
			if len(toks) > 0 && toks[0].Type == "DOLL" {
				toks = toks[:1]
			}
			acts := GetActs(toks, &sl)
			node_acts = append(node_acts, acts...)
			node_acts = append(node_acts, Action{Type: "GC"})
			TempN = 0
		}
		for n, nact := range node_acts {
			for m, v := range nact.Variables {
				if strings.HasPrefix(string(v), "\"") && strings.HasSuffix(string(v), "\"") {
					str, ok := string_map[string(v)[1:len(string(v))-1]]
					if !ok {
						continue
					}
					node_acts[n].Variables[m] = Variable("\"" + str + "\"")
				}
			}
		}
		compiled[key] = node_acts
	}
	return compiled
}

type Function struct {
	Name   string
	Target string
	Vars   []Variable
	Node   string
}

func GenerateFuns() []Function {
	strs := []string{"print", "out", "where", "len", "read", "write", "isdir", "exit", "type", "convert", "list", "array", "pair", "append", "system", "source", "run", "sort", "id", "ternary", "rand", "input", "glob", "env", "range", "fmt", "chdir", "split", "join", "to_upper", "to_lower", "cp", "mv", "rm", "pop", "itc", "cti", "has", "index", "replace", "re_match", "re_find", "rget", "rpost", "arrm", "value", "sub"}
	fs := []Function{}
	for _, str := range strs {
		fs = append(fs, Function{Name: str})
	}
	return fs
}

type Vars struct {
	Names  map[string]uint64
	Types  map[uint64]byte
	Ints   map[uint64]*big.Int
	Floats map[uint64]*big.Float
	Strs   map[uint64]string
	Bools  map[uint64]bool
	Bytes  map[uint64]byte
	Funcs  map[uint64]Function
	Ids    map[uint64]uint64
	Arrs   map[uint64]Array
	Spans  map[uint64]Span
	Lists  map[uint64]List
	Pairs  map[uint64]Pair
}

type List struct {
	Ids []*MinPtr
}

type Array struct {
	Dtype  byte
	Ints   []*big.Int
	Floats []*big.Float
	Strs   []string
	Bools  []bool
	Bytes  []byte
	Funcs  []Function
	Ids    []uint64
	Arrs   []Array
	Spans  []Span
	Lists  []List
	Pairs  []Pair
}

func (a *Array) Append(item any) error {
	switch v := item.(type) {
	case *big.Int:
		if a.Dtype == NOTH {
			a.Dtype = INT
		}
		if a.Dtype != INT {
			return fmt.Errorf("array of type id %d, appended item of type id %d", a.Dtype, INT)
		}
		a.Ints = append(a.Ints, v)
	case *big.Float:
		if a.Dtype == NOTH {
			a.Dtype = FLOAT
		}
		if a.Dtype != FLOAT {
			return fmt.Errorf("array of type id %d, appended item of type id %d", a.Dtype, FLOAT)
		}
		a.Floats = append(a.Floats, v)
	case string:
		if a.Dtype == NOTH {
			a.Dtype = STR
		}
		if a.Dtype != STR {
			return fmt.Errorf("array of type id %d, appended item of type id %d", a.Dtype, STR)
		}
		a.Strs = append(a.Strs, v)
	case bool:
		if a.Dtype == NOTH {
			a.Dtype = BOOL
		}
		if a.Dtype != BOOL {
			return fmt.Errorf("array of type id %d, appended item of type id %d", a.Dtype, BOOL)
		}
		a.Bools = append(a.Bools, v)
	case byte:
		if a.Dtype == NOTH {
			a.Dtype = BYTE
		}
		if a.Dtype != BYTE {
			return fmt.Errorf("array of type id %d, appended item of type id %d", a.Dtype, BYTE)
		}
		a.Bytes = append(a.Bytes, v)
	case Function:
		if a.Dtype == NOTH {
			a.Dtype = FUNC
		}
		if a.Dtype != FUNC {
			return fmt.Errorf("array of type id %d, appended item of type id %d", a.Dtype, FUNC)
		}
		a.Funcs = append(a.Funcs, v)
	case uint64:
		if a.Dtype == NOTH {
			a.Dtype = ID
		}
		if a.Dtype != ID {
			return fmt.Errorf("array of type id %d, appended item of type id %d", a.Dtype, ID)
		}
		a.Ids = append(a.Ids, v)
	case Array:
		if a.Dtype == NOTH {
			a.Dtype = ARR
		}
		if a.Dtype != ARR {
			return fmt.Errorf("array of type id %d, appended item of type id %d", a.Dtype, ARR)
		}
		a.Arrs = append(a.Arrs, v)
	case Span:
		if a.Dtype == NOTH {
			a.Dtype = SPAN
		}
		if a.Dtype != SPAN {
			return fmt.Errorf("array of type id %d, appended item of type id %d", a.Dtype, SPAN)
		}
		a.Spans = append(a.Spans, v)
	case List:
		if a.Dtype == NOTH {
			a.Dtype = LIST
		}
		if a.Dtype != LIST {
			return fmt.Errorf("array of type id %d, appended item of type id %d", a.Dtype, LIST)
		}
		a.Lists = append(a.Lists, v)
	case Pair:
		if a.Dtype == NOTH {
			a.Dtype = PAIR
		}
		if a.Dtype != PAIR {
			return fmt.Errorf("array of type id %d, appended item of type id %d", a.Dtype, PAIR)
		}
		a.Pairs = append(a.Pairs, v)
	default:
		return fmt.Errorf("illegal value: %v", v)
	}
	return nil
}

func (a *Array) String() string {
	dstrings := map[byte]string{0: "noth", 1: "int", 2: "float", 3: "str", 4: "arr", 5: "list", 6: "pair", 7: "bool", 8: "byte", 9: "func", 10: "id", 11: "arrm"}
	output := dstrings[a.Dtype] + ".["
	switch a.Dtype {
	case NOTH:
		return "noth.[]"
	case INT:
		for _, item := range a.Ints {
			output += item.String() + ", "
		}
	case FLOAT:
		for _, item := range a.Floats {
			output += item.String() + ", "
		}
	case BYTE:
		for _, item := range a.Bytes {
			output += fmt.Sprintf("b.%d, ", item)
		}
	case STR:
		for _, item := range a.Strs {
			output += fmt.Sprintf("\"%s\", ", item)
		}
	case BOOL:
		for _, item := range a.Bools {
			output += fmt.Sprintf("%v, ", item)
		}
	case FUNC:
		for _, item := range a.Funcs {
			output += fmt.Sprintf("func.%s, ", item.Name)
		}
	case LIST, PAIR, SPAN:
		// todo
	case ARR:
		for _, item := range a.Arrs {
			output += item.String()
		}
	case ID:
		for _, item := range a.Ids {
			output += fmt.Sprintf("%d, ", item)
		}
	}
	return output[:len(output)-2] + "]"
}

type Span struct {
	Dtype  byte
	Start  uint64
	Length uint64
}

type Pair struct {
	Ids map[string]*MinPtr
}
