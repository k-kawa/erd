package main

import (
	"strings"
	"fmt"
	"math"
	"sort"
	"strconv"
)

const endSymbol rune = 1114112

/* The rule types inferred from the grammar are below. */
type pegRule uint8

const (
	ruleUnknown pegRule = iota
	ruleroot
	ruleComment
	ruleSep
	ruleSpace
	ruleTableDef
	ruleLeftBrace
	ruleRightBrace
	ruleTableName
	ruleTableDescription
	ruleColumns
	ruleColumn
	ruleColumnDescription
	ruledot
	ruleColumnName
	ruleColumnDef
	ruleRightArrow
	ruleColumnType
	ruleRightDotArrow
	ruleRightLineArrow
	ruleTargetTableName
	ruleTargetColumnName
	ruleEOT
	ruleAction0
	rulePegText
	ruleAction1
	ruleAction2
	ruleAction3
	ruleAction4
	ruleAction5
	ruleAction6
	ruleAction7
	ruleAction8
	ruleAction9
	ruleAction10
)

var rul3s = [...]string{
	"Unknown",
	"root",
	"Comment",
	"Sep",
	"Space",
	"TableDef",
	"LeftBrace",
	"RightBrace",
	"TableName",
	"TableDescription",
	"Columns",
	"Column",
	"ColumnDescription",
	"dot",
	"ColumnName",
	"ColumnDef",
	"RightArrow",
	"ColumnType",
	"RightDotArrow",
	"RightLineArrow",
	"TargetTableName",
	"TargetColumnName",
	"EOT",
	"Action0",
	"PegText",
	"Action1",
	"Action2",
	"Action3",
	"Action4",
	"Action5",
	"Action6",
	"Action7",
	"Action8",
	"Action9",
	"Action10",
}

type token32 struct {
	pegRule
	begin, end uint32
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v", rul3s[t.pegRule], t.begin, t.end)
}

type node32 struct {
	token32
	up, next *node32
}

func (node *node32) print(pretty bool, buffer string) {
	var print func(node *node32, depth int)
	print = func(node *node32, depth int) {
		for node != nil {
			for c := 0; c < depth; c++ {
				fmt.Printf(" ")
			}
			rule := rul3s[node.pegRule]
			quote := strconv.Quote(string(([]rune(buffer)[node.begin:node.end])))
			if !pretty {
				fmt.Printf("%v %v\n", rule, quote)
			} else {
				fmt.Printf("\x1B[34m%v\x1B[m %v\n", rule, quote)
			}
			if node.up != nil {
				print(node.up, depth+1)
			}
			node = node.next
		}
	}
	print(node, 0)
}

func (node *node32) Print(buffer string) {
	node.print(false, buffer)
}

func (node *node32) PrettyPrint(buffer string) {
	node.print(true, buffer)
}

type tokens32 struct {
	tree []token32
}

func (t *tokens32) Trim(length uint32) {
	t.tree = t.tree[:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) AST() *node32 {
	type element struct {
		node *node32
		down *element
	}
	tokens := t.Tokens()
	var stack *element
	for _, token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	if stack != nil {
		return stack.node
	}
	return nil
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	t.AST().Print(buffer)
}

func (t *tokens32) PrettyPrintSyntaxTree(buffer string) {
	t.AST().PrettyPrint(buffer)
}

func (t *tokens32) Add(rule pegRule, begin, end, index uint32) {
	if tree := t.tree; int(index) >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	t.tree[index] = token32{
		pegRule: rule,
		begin:   begin,
		end:     end,
	}
}

func (t *tokens32) Tokens() []token32 {
	return t.tree
}

type Parser struct {
	tables []Table
	table  *Table
	column *Column

	Buffer string
	buffer []rune
	rules  [35]func() bool
	parse  func(rule ...int) error
	reset  func()
	Pretty bool
	tokens32
}

func (p *Parser) Parse(rule ...int) error {
	return p.parse(rule...)
}

func (p *Parser) Reset() {
	p.reset()
}

type textPosition struct {
	line, symbol int
}

type textPositionMap map[int]textPosition

func translatePositions(buffer []rune, positions []int) textPositionMap {
	length, translations, j, line, symbol := len(positions), make(textPositionMap, len(positions)), 0, 1, 0
	sort.Ints(positions)

search:
	for i, c := range buffer {
		if c == '\n' {
			line, symbol = line+1, 0
		} else {
			symbol++
		}
		if i == positions[j] {
			translations[positions[j]] = textPosition{line, symbol}
			for j++; j < length; j++ {
				if i != positions[j] {
					continue search
				}
			}
			break search
		}
	}

	return translations
}

type parseError struct {
	p   *Parser
	max token32
}

func (e *parseError) Error() string {
	tokens, error := []token32{e.max}, "\n"
	positions, p := make([]int, 2*len(tokens)), 0
	for _, token := range tokens {
		positions[p], p = int(token.begin), p+1
		positions[p], p = int(token.end), p+1
	}
	translations := translatePositions(e.p.buffer, positions)
	format := "parse error near %v (line %v symbol %v - line %v symbol %v):\n%v\n"
	if e.p.Pretty {
		format = "parse error near \x1B[34m%v\x1B[m (line %v symbol %v - line %v symbol %v):\n%v\n"
	}
	for _, token := range tokens {
		begin, end := int(token.begin), int(token.end)
		error += fmt.Sprintf(format,
			rul3s[token.pegRule],
			translations[begin].line, translations[begin].symbol,
			translations[end].line, translations[end].symbol,
			strconv.Quote(string(e.p.buffer[begin:end])))
	}

	return error
}

func (p *Parser) PrintSyntaxTree() {
	if p.Pretty {
		p.tokens32.PrettyPrintSyntaxTree(p.Buffer)
	} else {
		p.tokens32.PrintSyntaxTree(p.Buffer)
	}
}

func (p *Parser) Execute() {
	buffer, _buffer, text, begin, end := p.Buffer, p.buffer, "", 0, 0
	for _, token := range p.Tokens() {
		switch token.pegRule {

		case rulePegText:
			begin, end = int(token.begin), int(token.end)
			text = string(_buffer[begin:end])

		case ruleAction0:

			p.tables = append(p.tables, *p.table)

		case ruleAction1:

			p.table = &Table{
				Name:        text,
				Columns:     make([]Column, 0),
				Description: "",
			}

		case ruleAction2:

			p.table.Description = strings.TrimSpace(text)

		case ruleAction3:

			p.table.Columns = append(p.table.Columns, *p.column)

		case ruleAction4:

			p.column.Description = strings.TrimSpace(text)

		case ruleAction5:

			p.column = &Column{
				Name: text,
			}

		case ruleAction6:

			p.column.Type = strings.TrimSpace(text)

		case ruleAction7:

			p.column.Relation = &Relation{
				LineType: DotLine,
			}

		case ruleAction8:

			p.column.Relation = &Relation{
				LineType: NormalLine,
			}

		case ruleAction9:

			p.column.Relation.TableName = text

		case ruleAction10:

			p.column.Relation.ColumnName = text

		}
	}
	_, _, _, _, _ = buffer, _buffer, text, begin, end
}

func (p *Parser) Init() {
	var (
		max                  token32
		position, tokenIndex uint32
		buffer               []rune
	)
	p.reset = func() {
		max = token32{}
		position, tokenIndex = 0, 0

		p.buffer = []rune(p.Buffer)
		if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != endSymbol {
			p.buffer = append(p.buffer, endSymbol)
		}
		buffer = p.buffer
	}
	p.reset()

	_rules := p.rules
	tree := tokens32{tree: make([]token32, math.MaxInt16)}
	p.parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokens32 = tree
		if matches {
			p.Trim(tokenIndex)
			return nil
		}
		return &parseError{p, max}
	}

	add := func(rule pegRule, begin uint32) {
		tree.Add(rule, begin, position, tokenIndex)
		tokenIndex++
		if begin != position && position > max.end {
			max = token32{rule, begin, position}
		}
	}

	matchDot := func() bool {
		if buffer[position] != endSymbol {
			position++
			return true
		}
		return false
	}

	/*matchChar := func(c byte) bool {
		if buffer[position] == c {
			position++
			return true
		}
		return false
	}*/

	/*matchRange := func(lower byte, upper byte) bool {
		if c := buffer[position]; c >= lower && c <= upper {
			position++
			return true
		}
		return false
	}*/

	_rules = [...]func() bool{
		nil,
		/* 0 root <- <((Sep* TableDef)* Sep* EOT)> */
		func() bool {
			position0, tokenIndex0 := position, tokenIndex
			{
				position1 := position
			l2:
				{
					position3, tokenIndex3 := position, tokenIndex
				l4:
					{
						position5, tokenIndex5 := position, tokenIndex
						if !_rules[ruleSep]() {
							goto l5
						}
						goto l4
					l5:
						position, tokenIndex = position5, tokenIndex5
					}
					if !_rules[ruleTableDef]() {
						goto l3
					}
					goto l2
				l3:
					position, tokenIndex = position3, tokenIndex3
				}
			l6:
				{
					position7, tokenIndex7 := position, tokenIndex
					if !_rules[ruleSep]() {
						goto l7
					}
					goto l6
				l7:
					position, tokenIndex = position7, tokenIndex7
				}
				if !_rules[ruleEOT]() {
					goto l0
				}
				add(ruleroot, position1)
			}
			return true
		l0:
			position, tokenIndex = position0, tokenIndex0
			return false
		},
		/* 1 Comment <- <('\n' '#' (!'\n' .)+ '\n')> */
		func() bool {
			position8, tokenIndex8 := position, tokenIndex
			{
				position9 := position
				if buffer[position] != rune('\n') {
					goto l8
				}
				position++
				if buffer[position] != rune('#') {
					goto l8
				}
				position++
				{
					position12, tokenIndex12 := position, tokenIndex
					if buffer[position] != rune('\n') {
						goto l12
					}
					position++
					goto l8
				l12:
					position, tokenIndex = position12, tokenIndex12
				}
				if !matchDot() {
					goto l8
				}
			l10:
				{
					position11, tokenIndex11 := position, tokenIndex
					{
						position13, tokenIndex13 := position, tokenIndex
						if buffer[position] != rune('\n') {
							goto l13
						}
						position++
						goto l11
					l13:
						position, tokenIndex = position13, tokenIndex13
					}
					if !matchDot() {
						goto l11
					}
					goto l10
				l11:
					position, tokenIndex = position11, tokenIndex11
				}
				if buffer[position] != rune('\n') {
					goto l8
				}
				position++
				add(ruleComment, position9)
			}
			return true
		l8:
			position, tokenIndex = position8, tokenIndex8
			return false
		},
		/* 2 Sep <- <(Comment / ('\n' / '\t' / ' '))> */
		func() bool {
			position14, tokenIndex14 := position, tokenIndex
			{
				position15 := position
				{
					position16, tokenIndex16 := position, tokenIndex
					if !_rules[ruleComment]() {
						goto l17
					}
					goto l16
				l17:
					position, tokenIndex = position16, tokenIndex16
					{
						position18, tokenIndex18 := position, tokenIndex
						if buffer[position] != rune('\n') {
							goto l19
						}
						position++
						goto l18
					l19:
						position, tokenIndex = position18, tokenIndex18
						if buffer[position] != rune('\t') {
							goto l20
						}
						position++
						goto l18
					l20:
						position, tokenIndex = position18, tokenIndex18
						if buffer[position] != rune(' ') {
							goto l14
						}
						position++
					}
				l18:
				}
			l16:
				add(ruleSep, position15)
			}
			return true
		l14:
			position, tokenIndex = position14, tokenIndex14
			return false
		},
		/* 3 Space <- <' '> */
		func() bool {
			position21, tokenIndex21 := position, tokenIndex
			{
				position22 := position
				if buffer[position] != rune(' ') {
					goto l21
				}
				position++
				add(ruleSpace, position22)
			}
			return true
		l21:
			position, tokenIndex = position21, tokenIndex21
			return false
		},
		/* 4 TableDef <- <(TableName Sep+ (':' Space* TableDescription)? LeftBrace Sep+ Columns Sep+ RightBrace)> */
		func() bool {
			position23, tokenIndex23 := position, tokenIndex
			{
				position24 := position
				if !_rules[ruleTableName]() {
					goto l23
				}
				if !_rules[ruleSep]() {
					goto l23
				}
			l25:
				{
					position26, tokenIndex26 := position, tokenIndex
					if !_rules[ruleSep]() {
						goto l26
					}
					goto l25
				l26:
					position, tokenIndex = position26, tokenIndex26
				}
				{
					position27, tokenIndex27 := position, tokenIndex
					if buffer[position] != rune(':') {
						goto l27
					}
					position++
				l29:
					{
						position30, tokenIndex30 := position, tokenIndex
						if !_rules[ruleSpace]() {
							goto l30
						}
						goto l29
					l30:
						position, tokenIndex = position30, tokenIndex30
					}
					if !_rules[ruleTableDescription]() {
						goto l27
					}
					goto l28
				l27:
					position, tokenIndex = position27, tokenIndex27
				}
			l28:
				if !_rules[ruleLeftBrace]() {
					goto l23
				}
				if !_rules[ruleSep]() {
					goto l23
				}
			l31:
				{
					position32, tokenIndex32 := position, tokenIndex
					if !_rules[ruleSep]() {
						goto l32
					}
					goto l31
				l32:
					position, tokenIndex = position32, tokenIndex32
				}
				if !_rules[ruleColumns]() {
					goto l23
				}
				if !_rules[ruleSep]() {
					goto l23
				}
			l33:
				{
					position34, tokenIndex34 := position, tokenIndex
					if !_rules[ruleSep]() {
						goto l34
					}
					goto l33
				l34:
					position, tokenIndex = position34, tokenIndex34
				}
				if !_rules[ruleRightBrace]() {
					goto l23
				}
				add(ruleTableDef, position24)
			}
			return true
		l23:
			position, tokenIndex = position23, tokenIndex23
			return false
		},
		/* 5 LeftBrace <- <'{'> */
		func() bool {
			position35, tokenIndex35 := position, tokenIndex
			{
				position36 := position
				if buffer[position] != rune('{') {
					goto l35
				}
				position++
				add(ruleLeftBrace, position36)
			}
			return true
		l35:
			position, tokenIndex = position35, tokenIndex35
			return false
		},
		/* 6 RightBrace <- <('}' Action0)> */
		func() bool {
			position37, tokenIndex37 := position, tokenIndex
			{
				position38 := position
				if buffer[position] != rune('}') {
					goto l37
				}
				position++
				if !_rules[ruleAction0]() {
					goto l37
				}
				add(ruleRightBrace, position38)
			}
			return true
		l37:
			position, tokenIndex = position37, tokenIndex37
			return false
		},
		/* 7 TableName <- <(<([a-z] / [A-Z] / [0-9] / '_')+> Action1)> */
		func() bool {
			position39, tokenIndex39 := position, tokenIndex
			{
				position40 := position
				{
					position41 := position
					{
						position44, tokenIndex44 := position, tokenIndex
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l45
						}
						position++
						goto l44
					l45:
						position, tokenIndex = position44, tokenIndex44
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l46
						}
						position++
						goto l44
					l46:
						position, tokenIndex = position44, tokenIndex44
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l47
						}
						position++
						goto l44
					l47:
						position, tokenIndex = position44, tokenIndex44
						if buffer[position] != rune('_') {
							goto l39
						}
						position++
					}
				l44:
				l42:
					{
						position43, tokenIndex43 := position, tokenIndex
						{
							position48, tokenIndex48 := position, tokenIndex
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l49
							}
							position++
							goto l48
						l49:
							position, tokenIndex = position48, tokenIndex48
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l50
							}
							position++
							goto l48
						l50:
							position, tokenIndex = position48, tokenIndex48
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l51
							}
							position++
							goto l48
						l51:
							position, tokenIndex = position48, tokenIndex48
							if buffer[position] != rune('_') {
								goto l43
							}
							position++
						}
					l48:
						goto l42
					l43:
						position, tokenIndex = position43, tokenIndex43
					}
					add(rulePegText, position41)
				}
				if !_rules[ruleAction1]() {
					goto l39
				}
				add(ruleTableName, position40)
			}
			return true
		l39:
			position, tokenIndex = position39, tokenIndex39
			return false
		},
		/* 8 TableDescription <- <(<(!('\n' / '{') .)+> Action2)> */
		func() bool {
			position52, tokenIndex52 := position, tokenIndex
			{
				position53 := position
				{
					position54 := position
					{
						position57, tokenIndex57 := position, tokenIndex
						{
							position58, tokenIndex58 := position, tokenIndex
							if buffer[position] != rune('\n') {
								goto l59
							}
							position++
							goto l58
						l59:
							position, tokenIndex = position58, tokenIndex58
							if buffer[position] != rune('{') {
								goto l57
							}
							position++
						}
					l58:
						goto l52
					l57:
						position, tokenIndex = position57, tokenIndex57
					}
					if !matchDot() {
						goto l52
					}
				l55:
					{
						position56, tokenIndex56 := position, tokenIndex
						{
							position60, tokenIndex60 := position, tokenIndex
							{
								position61, tokenIndex61 := position, tokenIndex
								if buffer[position] != rune('\n') {
									goto l62
								}
								position++
								goto l61
							l62:
								position, tokenIndex = position61, tokenIndex61
								if buffer[position] != rune('{') {
									goto l60
								}
								position++
							}
						l61:
							goto l56
						l60:
							position, tokenIndex = position60, tokenIndex60
						}
						if !matchDot() {
							goto l56
						}
						goto l55
					l56:
						position, tokenIndex = position56, tokenIndex56
					}
					add(rulePegText, position54)
				}
				if !_rules[ruleAction2]() {
					goto l52
				}
				add(ruleTableDescription, position53)
			}
			return true
		l52:
			position, tokenIndex = position52, tokenIndex52
			return false
		},
		/* 9 Columns <- <(Column (Sep+ Column)*)> */
		func() bool {
			position63, tokenIndex63 := position, tokenIndex
			{
				position64 := position
				if !_rules[ruleColumn]() {
					goto l63
				}
			l65:
				{
					position66, tokenIndex66 := position, tokenIndex
					if !_rules[ruleSep]() {
						goto l66
					}
				l67:
					{
						position68, tokenIndex68 := position, tokenIndex
						if !_rules[ruleSep]() {
							goto l68
						}
						goto l67
					l68:
						position, tokenIndex = position68, tokenIndex68
					}
					if !_rules[ruleColumn]() {
						goto l66
					}
					goto l65
				l66:
					position, tokenIndex = position66, tokenIndex66
				}
				add(ruleColumns, position64)
			}
			return true
		l63:
			position, tokenIndex = position63, tokenIndex63
			return false
		},
		/* 10 Column <- <(ColumnDef Space* (RightArrow Sep+ TargetTableName dot TargetColumnName Space*)? (':' Space* ColumnDescription)? Action3)> */
		func() bool {
			position69, tokenIndex69 := position, tokenIndex
			{
				position70 := position
				if !_rules[ruleColumnDef]() {
					goto l69
				}
			l71:
				{
					position72, tokenIndex72 := position, tokenIndex
					if !_rules[ruleSpace]() {
						goto l72
					}
					goto l71
				l72:
					position, tokenIndex = position72, tokenIndex72
				}
				{
					position73, tokenIndex73 := position, tokenIndex
					if !_rules[ruleRightArrow]() {
						goto l73
					}
					if !_rules[ruleSep]() {
						goto l73
					}
				l75:
					{
						position76, tokenIndex76 := position, tokenIndex
						if !_rules[ruleSep]() {
							goto l76
						}
						goto l75
					l76:
						position, tokenIndex = position76, tokenIndex76
					}
					if !_rules[ruleTargetTableName]() {
						goto l73
					}
					if !_rules[ruledot]() {
						goto l73
					}
					if !_rules[ruleTargetColumnName]() {
						goto l73
					}
				l77:
					{
						position78, tokenIndex78 := position, tokenIndex
						if !_rules[ruleSpace]() {
							goto l78
						}
						goto l77
					l78:
						position, tokenIndex = position78, tokenIndex78
					}
					goto l74
				l73:
					position, tokenIndex = position73, tokenIndex73
				}
			l74:
				{
					position79, tokenIndex79 := position, tokenIndex
					if buffer[position] != rune(':') {
						goto l79
					}
					position++
				l81:
					{
						position82, tokenIndex82 := position, tokenIndex
						if !_rules[ruleSpace]() {
							goto l82
						}
						goto l81
					l82:
						position, tokenIndex = position82, tokenIndex82
					}
					if !_rules[ruleColumnDescription]() {
						goto l79
					}
					goto l80
				l79:
					position, tokenIndex = position79, tokenIndex79
				}
			l80:
				if !_rules[ruleAction3]() {
					goto l69
				}
				add(ruleColumn, position70)
			}
			return true
		l69:
			position, tokenIndex = position69, tokenIndex69
			return false
		},
		/* 11 ColumnDescription <- <(<(!'\n' .)+> Action4)> */
		func() bool {
			position83, tokenIndex83 := position, tokenIndex
			{
				position84 := position
				{
					position85 := position
					{
						position88, tokenIndex88 := position, tokenIndex
						if buffer[position] != rune('\n') {
							goto l88
						}
						position++
						goto l83
					l88:
						position, tokenIndex = position88, tokenIndex88
					}
					if !matchDot() {
						goto l83
					}
				l86:
					{
						position87, tokenIndex87 := position, tokenIndex
						{
							position89, tokenIndex89 := position, tokenIndex
							if buffer[position] != rune('\n') {
								goto l89
							}
							position++
							goto l87
						l89:
							position, tokenIndex = position89, tokenIndex89
						}
						if !matchDot() {
							goto l87
						}
						goto l86
					l87:
						position, tokenIndex = position87, tokenIndex87
					}
					add(rulePegText, position85)
				}
				if !_rules[ruleAction4]() {
					goto l83
				}
				add(ruleColumnDescription, position84)
			}
			return true
		l83:
			position, tokenIndex = position83, tokenIndex83
			return false
		},
		/* 12 dot <- <'.'> */
		func() bool {
			position90, tokenIndex90 := position, tokenIndex
			{
				position91 := position
				if buffer[position] != rune('.') {
					goto l90
				}
				position++
				add(ruledot, position91)
			}
			return true
		l90:
			position, tokenIndex = position90, tokenIndex90
			return false
		},
		/* 13 ColumnName <- <(<([a-z] / [A-Z] / [0-9] / '_')+> Action5)> */
		func() bool {
			position92, tokenIndex92 := position, tokenIndex
			{
				position93 := position
				{
					position94 := position
					{
						position97, tokenIndex97 := position, tokenIndex
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l98
						}
						position++
						goto l97
					l98:
						position, tokenIndex = position97, tokenIndex97
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l99
						}
						position++
						goto l97
					l99:
						position, tokenIndex = position97, tokenIndex97
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l100
						}
						position++
						goto l97
					l100:
						position, tokenIndex = position97, tokenIndex97
						if buffer[position] != rune('_') {
							goto l92
						}
						position++
					}
				l97:
				l95:
					{
						position96, tokenIndex96 := position, tokenIndex
						{
							position101, tokenIndex101 := position, tokenIndex
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l102
							}
							position++
							goto l101
						l102:
							position, tokenIndex = position101, tokenIndex101
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l103
							}
							position++
							goto l101
						l103:
							position, tokenIndex = position101, tokenIndex101
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l104
							}
							position++
							goto l101
						l104:
							position, tokenIndex = position101, tokenIndex101
							if buffer[position] != rune('_') {
								goto l96
							}
							position++
						}
					l101:
						goto l95
					l96:
						position, tokenIndex = position96, tokenIndex96
					}
					add(rulePegText, position94)
				}
				if !_rules[ruleAction5]() {
					goto l92
				}
				add(ruleColumnName, position93)
			}
			return true
		l92:
			position, tokenIndex = position92, tokenIndex92
			return false
		},
		/* 14 ColumnDef <- <(ColumnName (Space* ColumnType)?)> */
		func() bool {
			position105, tokenIndex105 := position, tokenIndex
			{
				position106 := position
				if !_rules[ruleColumnName]() {
					goto l105
				}
				{
					position107, tokenIndex107 := position, tokenIndex
				l109:
					{
						position110, tokenIndex110 := position, tokenIndex
						if !_rules[ruleSpace]() {
							goto l110
						}
						goto l109
					l110:
						position, tokenIndex = position110, tokenIndex110
					}
					if !_rules[ruleColumnType]() {
						goto l107
					}
					goto l108
				l107:
					position, tokenIndex = position107, tokenIndex107
				}
			l108:
				add(ruleColumnDef, position106)
			}
			return true
		l105:
			position, tokenIndex = position105, tokenIndex105
			return false
		},
		/* 15 RightArrow <- <(RightDotArrow / RightLineArrow)> */
		func() bool {
			position111, tokenIndex111 := position, tokenIndex
			{
				position112 := position
				{
					position113, tokenIndex113 := position, tokenIndex
					if !_rules[ruleRightDotArrow]() {
						goto l114
					}
					goto l113
				l114:
					position, tokenIndex = position113, tokenIndex113
					if !_rules[ruleRightLineArrow]() {
						goto l111
					}
				}
			l113:
				add(ruleRightArrow, position112)
			}
			return true
		l111:
			position, tokenIndex = position111, tokenIndex111
			return false
		},
		/* 16 ColumnType <- <(<(!('-' / ':' / '.' / '\n') .)+> Action6)> */
		func() bool {
			position115, tokenIndex115 := position, tokenIndex
			{
				position116 := position
				{
					position117 := position
					{
						position120, tokenIndex120 := position, tokenIndex
						{
							position121, tokenIndex121 := position, tokenIndex
							if buffer[position] != rune('-') {
								goto l122
							}
							position++
							goto l121
						l122:
							position, tokenIndex = position121, tokenIndex121
							if buffer[position] != rune(':') {
								goto l123
							}
							position++
							goto l121
						l123:
							position, tokenIndex = position121, tokenIndex121
							if buffer[position] != rune('.') {
								goto l124
							}
							position++
							goto l121
						l124:
							position, tokenIndex = position121, tokenIndex121
							if buffer[position] != rune('\n') {
								goto l120
							}
							position++
						}
					l121:
						goto l115
					l120:
						position, tokenIndex = position120, tokenIndex120
					}
					if !matchDot() {
						goto l115
					}
				l118:
					{
						position119, tokenIndex119 := position, tokenIndex
						{
							position125, tokenIndex125 := position, tokenIndex
							{
								position126, tokenIndex126 := position, tokenIndex
								if buffer[position] != rune('-') {
									goto l127
								}
								position++
								goto l126
							l127:
								position, tokenIndex = position126, tokenIndex126
								if buffer[position] != rune(':') {
									goto l128
								}
								position++
								goto l126
							l128:
								position, tokenIndex = position126, tokenIndex126
								if buffer[position] != rune('.') {
									goto l129
								}
								position++
								goto l126
							l129:
								position, tokenIndex = position126, tokenIndex126
								if buffer[position] != rune('\n') {
									goto l125
								}
								position++
							}
						l126:
							goto l119
						l125:
							position, tokenIndex = position125, tokenIndex125
						}
						if !matchDot() {
							goto l119
						}
						goto l118
					l119:
						position, tokenIndex = position119, tokenIndex119
					}
					add(rulePegText, position117)
				}
				if !_rules[ruleAction6]() {
					goto l115
				}
				add(ruleColumnType, position116)
			}
			return true
		l115:
			position, tokenIndex = position115, tokenIndex115
			return false
		},
		/* 17 RightDotArrow <- <('.' '.' '>' Action7)> */
		func() bool {
			position130, tokenIndex130 := position, tokenIndex
			{
				position131 := position
				if buffer[position] != rune('.') {
					goto l130
				}
				position++
				if buffer[position] != rune('.') {
					goto l130
				}
				position++
				if buffer[position] != rune('>') {
					goto l130
				}
				position++
				if !_rules[ruleAction7]() {
					goto l130
				}
				add(ruleRightDotArrow, position131)
			}
			return true
		l130:
			position, tokenIndex = position130, tokenIndex130
			return false
		},
		/* 18 RightLineArrow <- <('-' '>' Action8)> */
		func() bool {
			position132, tokenIndex132 := position, tokenIndex
			{
				position133 := position
				if buffer[position] != rune('-') {
					goto l132
				}
				position++
				if buffer[position] != rune('>') {
					goto l132
				}
				position++
				if !_rules[ruleAction8]() {
					goto l132
				}
				add(ruleRightLineArrow, position133)
			}
			return true
		l132:
			position, tokenIndex = position132, tokenIndex132
			return false
		},
		/* 19 TargetTableName <- <(<([a-z] / [A-Z] / [0-9] / '_')+> Action9)> */
		func() bool {
			position134, tokenIndex134 := position, tokenIndex
			{
				position135 := position
				{
					position136 := position
					{
						position139, tokenIndex139 := position, tokenIndex
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l140
						}
						position++
						goto l139
					l140:
						position, tokenIndex = position139, tokenIndex139
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l141
						}
						position++
						goto l139
					l141:
						position, tokenIndex = position139, tokenIndex139
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l142
						}
						position++
						goto l139
					l142:
						position, tokenIndex = position139, tokenIndex139
						if buffer[position] != rune('_') {
							goto l134
						}
						position++
					}
				l139:
				l137:
					{
						position138, tokenIndex138 := position, tokenIndex
						{
							position143, tokenIndex143 := position, tokenIndex
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l144
							}
							position++
							goto l143
						l144:
							position, tokenIndex = position143, tokenIndex143
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l145
							}
							position++
							goto l143
						l145:
							position, tokenIndex = position143, tokenIndex143
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l146
							}
							position++
							goto l143
						l146:
							position, tokenIndex = position143, tokenIndex143
							if buffer[position] != rune('_') {
								goto l138
							}
							position++
						}
					l143:
						goto l137
					l138:
						position, tokenIndex = position138, tokenIndex138
					}
					add(rulePegText, position136)
				}
				if !_rules[ruleAction9]() {
					goto l134
				}
				add(ruleTargetTableName, position135)
			}
			return true
		l134:
			position, tokenIndex = position134, tokenIndex134
			return false
		},
		/* 20 TargetColumnName <- <(<([a-z] / [A-Z] / [0-9] / '_')+> Action10)> */
		func() bool {
			position147, tokenIndex147 := position, tokenIndex
			{
				position148 := position
				{
					position149 := position
					{
						position152, tokenIndex152 := position, tokenIndex
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l153
						}
						position++
						goto l152
					l153:
						position, tokenIndex = position152, tokenIndex152
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l154
						}
						position++
						goto l152
					l154:
						position, tokenIndex = position152, tokenIndex152
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l155
						}
						position++
						goto l152
					l155:
						position, tokenIndex = position152, tokenIndex152
						if buffer[position] != rune('_') {
							goto l147
						}
						position++
					}
				l152:
				l150:
					{
						position151, tokenIndex151 := position, tokenIndex
						{
							position156, tokenIndex156 := position, tokenIndex
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l157
							}
							position++
							goto l156
						l157:
							position, tokenIndex = position156, tokenIndex156
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l158
							}
							position++
							goto l156
						l158:
							position, tokenIndex = position156, tokenIndex156
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l159
							}
							position++
							goto l156
						l159:
							position, tokenIndex = position156, tokenIndex156
							if buffer[position] != rune('_') {
								goto l151
							}
							position++
						}
					l156:
						goto l150
					l151:
						position, tokenIndex = position151, tokenIndex151
					}
					add(rulePegText, position149)
				}
				if !_rules[ruleAction10]() {
					goto l147
				}
				add(ruleTargetColumnName, position148)
			}
			return true
		l147:
			position, tokenIndex = position147, tokenIndex147
			return false
		},
		/* 21 EOT <- <!.> */
		func() bool {
			position160, tokenIndex160 := position, tokenIndex
			{
				position161 := position
				{
					position162, tokenIndex162 := position, tokenIndex
					if !matchDot() {
						goto l162
					}
					goto l160
				l162:
					position, tokenIndex = position162, tokenIndex162
				}
				add(ruleEOT, position161)
			}
			return true
		l160:
			position, tokenIndex = position160, tokenIndex160
			return false
		},
		/* 23 Action0 <- <{
		    p.tables = append(p.tables, *p.table)
		}> */
		func() bool {
			{
				add(ruleAction0, position)
			}
			return true
		},
		nil,
		/* 25 Action1 <- <{
		    p.table = &Table{
		        Name: text,
		        Columns: make([]Column, 0),
		        Description: "",
			   }
		}> */
		func() bool {
			{
				add(ruleAction1, position)
			}
			return true
		},
		/* 26 Action2 <- <{
		    p.table.Description = strings.TrimSpace(text)
		}> */
		func() bool {
			{
				add(ruleAction2, position)
			}
			return true
		},
		/* 27 Action3 <- <{
		    p.table.Columns = append(p.table.Columns, *p.column)
		}> */
		func() bool {
			{
				add(ruleAction3, position)
			}
			return true
		},
		/* 28 Action4 <- <{
		    p.column.Description = strings.TrimSpace(text)
		}> */
		func() bool {
			{
				add(ruleAction4, position)
			}
			return true
		},
		/* 29 Action5 <- <{
			p.column = &Column{
			  Name: text,
			}
		}> */
		func() bool {
			{
				add(ruleAction5, position)
			}
			return true
		},
		/* 30 Action6 <- <{
		    p.column.Type = strings.TrimSpace(text)
		}> */
		func() bool {
			{
				add(ruleAction6, position)
			}
			return true
		},
		/* 31 Action7 <- <{
		    p.column.Relation = &Relation{
		        LineType: DotLine,
		    }
		}> */
		func() bool {
			{
				add(ruleAction7, position)
			}
			return true
		},
		/* 32 Action8 <- <{
		    p.column.Relation = &Relation{
		        LineType: NormalLine,
		    }
		}> */
		func() bool {
			{
				add(ruleAction8, position)
			}
			return true
		},
		/* 33 Action9 <- <{
		    p.column.Relation.TableName = text
		}> */
		func() bool {
			{
				add(ruleAction9, position)
			}
			return true
		},
		/* 34 Action10 <- <{
		    p.column.Relation.ColumnName = text
		}> */
		func() bool {
			{
				add(ruleAction10, position)
			}
			return true
		},
	}
	p.rules = _rules
}
