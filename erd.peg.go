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
	rules  [34]func() bool
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
		/* 1 Sep <- <('\n' / '\t' / ' ')+> */
		func() bool {
			position8, tokenIndex8 := position, tokenIndex
			{
				position9 := position
				{
					position12, tokenIndex12 := position, tokenIndex
					if buffer[position] != rune('\n') {
						goto l13
					}
					position++
					goto l12
				l13:
					position, tokenIndex = position12, tokenIndex12
					if buffer[position] != rune('\t') {
						goto l14
					}
					position++
					goto l12
				l14:
					position, tokenIndex = position12, tokenIndex12
					if buffer[position] != rune(' ') {
						goto l8
					}
					position++
				}
			l12:
			l10:
				{
					position11, tokenIndex11 := position, tokenIndex
					{
						position15, tokenIndex15 := position, tokenIndex
						if buffer[position] != rune('\n') {
							goto l16
						}
						position++
						goto l15
					l16:
						position, tokenIndex = position15, tokenIndex15
						if buffer[position] != rune('\t') {
							goto l17
						}
						position++
						goto l15
					l17:
						position, tokenIndex = position15, tokenIndex15
						if buffer[position] != rune(' ') {
							goto l11
						}
						position++
					}
				l15:
					goto l10
				l11:
					position, tokenIndex = position11, tokenIndex11
				}
				add(ruleSep, position9)
			}
			return true
		l8:
			position, tokenIndex = position8, tokenIndex8
			return false
		},
		/* 2 Space <- <' '> */
		func() bool {
			position18, tokenIndex18 := position, tokenIndex
			{
				position19 := position
				if buffer[position] != rune(' ') {
					goto l18
				}
				position++
				add(ruleSpace, position19)
			}
			return true
		l18:
			position, tokenIndex = position18, tokenIndex18
			return false
		},
		/* 3 TableDef <- <(TableName Sep (':' Space* TableDescription)? LeftBrace Sep Columns Sep RightBrace)> */
		func() bool {
			position20, tokenIndex20 := position, tokenIndex
			{
				position21 := position
				if !_rules[ruleTableName]() {
					goto l20
				}
				if !_rules[ruleSep]() {
					goto l20
				}
				{
					position22, tokenIndex22 := position, tokenIndex
					if buffer[position] != rune(':') {
						goto l22
					}
					position++
				l24:
					{
						position25, tokenIndex25 := position, tokenIndex
						if !_rules[ruleSpace]() {
							goto l25
						}
						goto l24
					l25:
						position, tokenIndex = position25, tokenIndex25
					}
					if !_rules[ruleTableDescription]() {
						goto l22
					}
					goto l23
				l22:
					position, tokenIndex = position22, tokenIndex22
				}
			l23:
				if !_rules[ruleLeftBrace]() {
					goto l20
				}
				if !_rules[ruleSep]() {
					goto l20
				}
				if !_rules[ruleColumns]() {
					goto l20
				}
				if !_rules[ruleSep]() {
					goto l20
				}
				if !_rules[ruleRightBrace]() {
					goto l20
				}
				add(ruleTableDef, position21)
			}
			return true
		l20:
			position, tokenIndex = position20, tokenIndex20
			return false
		},
		/* 4 LeftBrace <- <'{'> */
		func() bool {
			position26, tokenIndex26 := position, tokenIndex
			{
				position27 := position
				if buffer[position] != rune('{') {
					goto l26
				}
				position++
				add(ruleLeftBrace, position27)
			}
			return true
		l26:
			position, tokenIndex = position26, tokenIndex26
			return false
		},
		/* 5 RightBrace <- <('}' Action0)> */
		func() bool {
			position28, tokenIndex28 := position, tokenIndex
			{
				position29 := position
				if buffer[position] != rune('}') {
					goto l28
				}
				position++
				if !_rules[ruleAction0]() {
					goto l28
				}
				add(ruleRightBrace, position29)
			}
			return true
		l28:
			position, tokenIndex = position28, tokenIndex28
			return false
		},
		/* 6 TableName <- <(<([a-z] / [A-Z] / [0-9] / '_')+> Action1)> */
		func() bool {
			position30, tokenIndex30 := position, tokenIndex
			{
				position31 := position
				{
					position32 := position
					{
						position35, tokenIndex35 := position, tokenIndex
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l36
						}
						position++
						goto l35
					l36:
						position, tokenIndex = position35, tokenIndex35
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l37
						}
						position++
						goto l35
					l37:
						position, tokenIndex = position35, tokenIndex35
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l38
						}
						position++
						goto l35
					l38:
						position, tokenIndex = position35, tokenIndex35
						if buffer[position] != rune('_') {
							goto l30
						}
						position++
					}
				l35:
				l33:
					{
						position34, tokenIndex34 := position, tokenIndex
						{
							position39, tokenIndex39 := position, tokenIndex
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l40
							}
							position++
							goto l39
						l40:
							position, tokenIndex = position39, tokenIndex39
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l41
							}
							position++
							goto l39
						l41:
							position, tokenIndex = position39, tokenIndex39
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l42
							}
							position++
							goto l39
						l42:
							position, tokenIndex = position39, tokenIndex39
							if buffer[position] != rune('_') {
								goto l34
							}
							position++
						}
					l39:
						goto l33
					l34:
						position, tokenIndex = position34, tokenIndex34
					}
					add(rulePegText, position32)
				}
				if !_rules[ruleAction1]() {
					goto l30
				}
				add(ruleTableName, position31)
			}
			return true
		l30:
			position, tokenIndex = position30, tokenIndex30
			return false
		},
		/* 7 TableDescription <- <(<(!('\n' / '{') .)+> Action2)> */
		func() bool {
			position43, tokenIndex43 := position, tokenIndex
			{
				position44 := position
				{
					position45 := position
					{
						position48, tokenIndex48 := position, tokenIndex
						{
							position49, tokenIndex49 := position, tokenIndex
							if buffer[position] != rune('\n') {
								goto l50
							}
							position++
							goto l49
						l50:
							position, tokenIndex = position49, tokenIndex49
							if buffer[position] != rune('{') {
								goto l48
							}
							position++
						}
					l49:
						goto l43
					l48:
						position, tokenIndex = position48, tokenIndex48
					}
					if !matchDot() {
						goto l43
					}
				l46:
					{
						position47, tokenIndex47 := position, tokenIndex
						{
							position51, tokenIndex51 := position, tokenIndex
							{
								position52, tokenIndex52 := position, tokenIndex
								if buffer[position] != rune('\n') {
									goto l53
								}
								position++
								goto l52
							l53:
								position, tokenIndex = position52, tokenIndex52
								if buffer[position] != rune('{') {
									goto l51
								}
								position++
							}
						l52:
							goto l47
						l51:
							position, tokenIndex = position51, tokenIndex51
						}
						if !matchDot() {
							goto l47
						}
						goto l46
					l47:
						position, tokenIndex = position47, tokenIndex47
					}
					add(rulePegText, position45)
				}
				if !_rules[ruleAction2]() {
					goto l43
				}
				add(ruleTableDescription, position44)
			}
			return true
		l43:
			position, tokenIndex = position43, tokenIndex43
			return false
		},
		/* 8 Columns <- <(Column (Sep Column)*)> */
		func() bool {
			position54, tokenIndex54 := position, tokenIndex
			{
				position55 := position
				if !_rules[ruleColumn]() {
					goto l54
				}
			l56:
				{
					position57, tokenIndex57 := position, tokenIndex
					if !_rules[ruleSep]() {
						goto l57
					}
					if !_rules[ruleColumn]() {
						goto l57
					}
					goto l56
				l57:
					position, tokenIndex = position57, tokenIndex57
				}
				add(ruleColumns, position55)
			}
			return true
		l54:
			position, tokenIndex = position54, tokenIndex54
			return false
		},
		/* 9 Column <- <(ColumnDef Space* (RightArrow Sep TargetTableName dot TargetColumnName Space*)? (':' Space* ColumnDescription)? Action3)> */
		func() bool {
			position58, tokenIndex58 := position, tokenIndex
			{
				position59 := position
				if !_rules[ruleColumnDef]() {
					goto l58
				}
			l60:
				{
					position61, tokenIndex61 := position, tokenIndex
					if !_rules[ruleSpace]() {
						goto l61
					}
					goto l60
				l61:
					position, tokenIndex = position61, tokenIndex61
				}
				{
					position62, tokenIndex62 := position, tokenIndex
					if !_rules[ruleRightArrow]() {
						goto l62
					}
					if !_rules[ruleSep]() {
						goto l62
					}
					if !_rules[ruleTargetTableName]() {
						goto l62
					}
					if !_rules[ruledot]() {
						goto l62
					}
					if !_rules[ruleTargetColumnName]() {
						goto l62
					}
				l64:
					{
						position65, tokenIndex65 := position, tokenIndex
						if !_rules[ruleSpace]() {
							goto l65
						}
						goto l64
					l65:
						position, tokenIndex = position65, tokenIndex65
					}
					goto l63
				l62:
					position, tokenIndex = position62, tokenIndex62
				}
			l63:
				{
					position66, tokenIndex66 := position, tokenIndex
					if buffer[position] != rune(':') {
						goto l66
					}
					position++
				l68:
					{
						position69, tokenIndex69 := position, tokenIndex
						if !_rules[ruleSpace]() {
							goto l69
						}
						goto l68
					l69:
						position, tokenIndex = position69, tokenIndex69
					}
					if !_rules[ruleColumnDescription]() {
						goto l66
					}
					goto l67
				l66:
					position, tokenIndex = position66, tokenIndex66
				}
			l67:
				if !_rules[ruleAction3]() {
					goto l58
				}
				add(ruleColumn, position59)
			}
			return true
		l58:
			position, tokenIndex = position58, tokenIndex58
			return false
		},
		/* 10 ColumnDescription <- <(<(!'\n' .)+> Action4)> */
		func() bool {
			position70, tokenIndex70 := position, tokenIndex
			{
				position71 := position
				{
					position72 := position
					{
						position75, tokenIndex75 := position, tokenIndex
						if buffer[position] != rune('\n') {
							goto l75
						}
						position++
						goto l70
					l75:
						position, tokenIndex = position75, tokenIndex75
					}
					if !matchDot() {
						goto l70
					}
				l73:
					{
						position74, tokenIndex74 := position, tokenIndex
						{
							position76, tokenIndex76 := position, tokenIndex
							if buffer[position] != rune('\n') {
								goto l76
							}
							position++
							goto l74
						l76:
							position, tokenIndex = position76, tokenIndex76
						}
						if !matchDot() {
							goto l74
						}
						goto l73
					l74:
						position, tokenIndex = position74, tokenIndex74
					}
					add(rulePegText, position72)
				}
				if !_rules[ruleAction4]() {
					goto l70
				}
				add(ruleColumnDescription, position71)
			}
			return true
		l70:
			position, tokenIndex = position70, tokenIndex70
			return false
		},
		/* 11 dot <- <'.'> */
		func() bool {
			position77, tokenIndex77 := position, tokenIndex
			{
				position78 := position
				if buffer[position] != rune('.') {
					goto l77
				}
				position++
				add(ruledot, position78)
			}
			return true
		l77:
			position, tokenIndex = position77, tokenIndex77
			return false
		},
		/* 12 ColumnName <- <(<([a-z] / [A-Z] / [0-9] / '_')+> Action5)> */
		func() bool {
			position79, tokenIndex79 := position, tokenIndex
			{
				position80 := position
				{
					position81 := position
					{
						position84, tokenIndex84 := position, tokenIndex
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l85
						}
						position++
						goto l84
					l85:
						position, tokenIndex = position84, tokenIndex84
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l86
						}
						position++
						goto l84
					l86:
						position, tokenIndex = position84, tokenIndex84
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l87
						}
						position++
						goto l84
					l87:
						position, tokenIndex = position84, tokenIndex84
						if buffer[position] != rune('_') {
							goto l79
						}
						position++
					}
				l84:
				l82:
					{
						position83, tokenIndex83 := position, tokenIndex
						{
							position88, tokenIndex88 := position, tokenIndex
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l89
							}
							position++
							goto l88
						l89:
							position, tokenIndex = position88, tokenIndex88
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l90
							}
							position++
							goto l88
						l90:
							position, tokenIndex = position88, tokenIndex88
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l91
							}
							position++
							goto l88
						l91:
							position, tokenIndex = position88, tokenIndex88
							if buffer[position] != rune('_') {
								goto l83
							}
							position++
						}
					l88:
						goto l82
					l83:
						position, tokenIndex = position83, tokenIndex83
					}
					add(rulePegText, position81)
				}
				if !_rules[ruleAction5]() {
					goto l79
				}
				add(ruleColumnName, position80)
			}
			return true
		l79:
			position, tokenIndex = position79, tokenIndex79
			return false
		},
		/* 13 ColumnDef <- <(ColumnName (Space* ColumnType)?)> */
		func() bool {
			position92, tokenIndex92 := position, tokenIndex
			{
				position93 := position
				if !_rules[ruleColumnName]() {
					goto l92
				}
				{
					position94, tokenIndex94 := position, tokenIndex
				l96:
					{
						position97, tokenIndex97 := position, tokenIndex
						if !_rules[ruleSpace]() {
							goto l97
						}
						goto l96
					l97:
						position, tokenIndex = position97, tokenIndex97
					}
					if !_rules[ruleColumnType]() {
						goto l94
					}
					goto l95
				l94:
					position, tokenIndex = position94, tokenIndex94
				}
			l95:
				add(ruleColumnDef, position93)
			}
			return true
		l92:
			position, tokenIndex = position92, tokenIndex92
			return false
		},
		/* 14 RightArrow <- <(RightDotArrow / RightLineArrow)> */
		func() bool {
			position98, tokenIndex98 := position, tokenIndex
			{
				position99 := position
				{
					position100, tokenIndex100 := position, tokenIndex
					if !_rules[ruleRightDotArrow]() {
						goto l101
					}
					goto l100
				l101:
					position, tokenIndex = position100, tokenIndex100
					if !_rules[ruleRightLineArrow]() {
						goto l98
					}
				}
			l100:
				add(ruleRightArrow, position99)
			}
			return true
		l98:
			position, tokenIndex = position98, tokenIndex98
			return false
		},
		/* 15 ColumnType <- <(<(!('-' / ':' / '.' / '\n') .)+> Action6)> */
		func() bool {
			position102, tokenIndex102 := position, tokenIndex
			{
				position103 := position
				{
					position104 := position
					{
						position107, tokenIndex107 := position, tokenIndex
						{
							position108, tokenIndex108 := position, tokenIndex
							if buffer[position] != rune('-') {
								goto l109
							}
							position++
							goto l108
						l109:
							position, tokenIndex = position108, tokenIndex108
							if buffer[position] != rune(':') {
								goto l110
							}
							position++
							goto l108
						l110:
							position, tokenIndex = position108, tokenIndex108
							if buffer[position] != rune('.') {
								goto l111
							}
							position++
							goto l108
						l111:
							position, tokenIndex = position108, tokenIndex108
							if buffer[position] != rune('\n') {
								goto l107
							}
							position++
						}
					l108:
						goto l102
					l107:
						position, tokenIndex = position107, tokenIndex107
					}
					if !matchDot() {
						goto l102
					}
				l105:
					{
						position106, tokenIndex106 := position, tokenIndex
						{
							position112, tokenIndex112 := position, tokenIndex
							{
								position113, tokenIndex113 := position, tokenIndex
								if buffer[position] != rune('-') {
									goto l114
								}
								position++
								goto l113
							l114:
								position, tokenIndex = position113, tokenIndex113
								if buffer[position] != rune(':') {
									goto l115
								}
								position++
								goto l113
							l115:
								position, tokenIndex = position113, tokenIndex113
								if buffer[position] != rune('.') {
									goto l116
								}
								position++
								goto l113
							l116:
								position, tokenIndex = position113, tokenIndex113
								if buffer[position] != rune('\n') {
									goto l112
								}
								position++
							}
						l113:
							goto l106
						l112:
							position, tokenIndex = position112, tokenIndex112
						}
						if !matchDot() {
							goto l106
						}
						goto l105
					l106:
						position, tokenIndex = position106, tokenIndex106
					}
					add(rulePegText, position104)
				}
				if !_rules[ruleAction6]() {
					goto l102
				}
				add(ruleColumnType, position103)
			}
			return true
		l102:
			position, tokenIndex = position102, tokenIndex102
			return false
		},
		/* 16 RightDotArrow <- <('.' '.' '>' Action7)> */
		func() bool {
			position117, tokenIndex117 := position, tokenIndex
			{
				position118 := position
				if buffer[position] != rune('.') {
					goto l117
				}
				position++
				if buffer[position] != rune('.') {
					goto l117
				}
				position++
				if buffer[position] != rune('>') {
					goto l117
				}
				position++
				if !_rules[ruleAction7]() {
					goto l117
				}
				add(ruleRightDotArrow, position118)
			}
			return true
		l117:
			position, tokenIndex = position117, tokenIndex117
			return false
		},
		/* 17 RightLineArrow <- <('-' '>' Action8)> */
		func() bool {
			position119, tokenIndex119 := position, tokenIndex
			{
				position120 := position
				if buffer[position] != rune('-') {
					goto l119
				}
				position++
				if buffer[position] != rune('>') {
					goto l119
				}
				position++
				if !_rules[ruleAction8]() {
					goto l119
				}
				add(ruleRightLineArrow, position120)
			}
			return true
		l119:
			position, tokenIndex = position119, tokenIndex119
			return false
		},
		/* 18 TargetTableName <- <(<([a-z] / [A-Z] / [0-9] / '_')+> Action9)> */
		func() bool {
			position121, tokenIndex121 := position, tokenIndex
			{
				position122 := position
				{
					position123 := position
					{
						position126, tokenIndex126 := position, tokenIndex
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l127
						}
						position++
						goto l126
					l127:
						position, tokenIndex = position126, tokenIndex126
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l128
						}
						position++
						goto l126
					l128:
						position, tokenIndex = position126, tokenIndex126
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l129
						}
						position++
						goto l126
					l129:
						position, tokenIndex = position126, tokenIndex126
						if buffer[position] != rune('_') {
							goto l121
						}
						position++
					}
				l126:
				l124:
					{
						position125, tokenIndex125 := position, tokenIndex
						{
							position130, tokenIndex130 := position, tokenIndex
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l131
							}
							position++
							goto l130
						l131:
							position, tokenIndex = position130, tokenIndex130
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l132
							}
							position++
							goto l130
						l132:
							position, tokenIndex = position130, tokenIndex130
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l133
							}
							position++
							goto l130
						l133:
							position, tokenIndex = position130, tokenIndex130
							if buffer[position] != rune('_') {
								goto l125
							}
							position++
						}
					l130:
						goto l124
					l125:
						position, tokenIndex = position125, tokenIndex125
					}
					add(rulePegText, position123)
				}
				if !_rules[ruleAction9]() {
					goto l121
				}
				add(ruleTargetTableName, position122)
			}
			return true
		l121:
			position, tokenIndex = position121, tokenIndex121
			return false
		},
		/* 19 TargetColumnName <- <(<([a-z] / [A-Z] / [0-9] / '_')+> Action10)> */
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
				if !_rules[ruleAction10]() {
					goto l134
				}
				add(ruleTargetColumnName, position135)
			}
			return true
		l134:
			position, tokenIndex = position134, tokenIndex134
			return false
		},
		/* 20 EOT <- <!.> */
		func() bool {
			position147, tokenIndex147 := position, tokenIndex
			{
				position148 := position
				{
					position149, tokenIndex149 := position, tokenIndex
					if !matchDot() {
						goto l149
					}
					goto l147
				l149:
					position, tokenIndex = position149, tokenIndex149
				}
				add(ruleEOT, position148)
			}
			return true
		l147:
			position, tokenIndex = position147, tokenIndex147
			return false
		},
		/* 22 Action0 <- <{
		    p.tables = append(p.tables, *p.table)
		}> */
		func() bool {
			{
				add(ruleAction0, position)
			}
			return true
		},
		nil,
		/* 24 Action1 <- <{
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
		/* 25 Action2 <- <{
		    p.table.Description = strings.TrimSpace(text)
		}> */
		func() bool {
			{
				add(ruleAction2, position)
			}
			return true
		},
		/* 26 Action3 <- <{
		    p.table.Columns = append(p.table.Columns, *p.column)
		}> */
		func() bool {
			{
				add(ruleAction3, position)
			}
			return true
		},
		/* 27 Action4 <- <{
		    p.column.Description = strings.TrimSpace(text)
		}> */
		func() bool {
			{
				add(ruleAction4, position)
			}
			return true
		},
		/* 28 Action5 <- <{
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
		/* 29 Action6 <- <{
		    p.column.Type = strings.TrimSpace(text)
		}> */
		func() bool {
			{
				add(ruleAction6, position)
			}
			return true
		},
		/* 30 Action7 <- <{
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
		/* 31 Action8 <- <{
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
		/* 32 Action9 <- <{
		    p.column.Relation.TableName = text
		}> */
		func() bool {
			{
				add(ruleAction9, position)
			}
			return true
		},
		/* 33 Action10 <- <{
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
