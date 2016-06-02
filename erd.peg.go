package main

import (
	"strings"
	"fmt"
	"math"
	"sort"
	"strconv"
)

const end_symbol rune = 1114112

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

	rulePre_
	rule_In_
	rule_Suf
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

	"Pre_",
	"_In_",
	"_Suf",
}

type tokenTree interface {
	Print()
	PrintSyntax()
	PrintSyntaxTree(buffer string)
	Add(rule pegRule, begin, end, next uint32, depth int)
	Expand(index int) tokenTree
	Tokens() <-chan token32
	AST() *node32
	Error() []token32
	trim(length int)
}

type node32 struct {
	token32
	up, next *node32
}

func (node *node32) print(depth int, buffer string) {
	for node != nil {
		for c := 0; c < depth; c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[node.pegRule], strconv.Quote(string(([]rune(buffer)[node.begin:node.end]))))
		if node.up != nil {
			node.up.print(depth+1, buffer)
		}
		node = node.next
	}
}

func (ast *node32) Print(buffer string) {
	ast.print(0, buffer)
}

type element struct {
	node *node32
	down *element
}

/* ${@} bit structure for abstract syntax tree */
type token32 struct {
	pegRule
	begin, end, next uint32
}

func (t *token32) isZero() bool {
	return t.pegRule == ruleUnknown && t.begin == 0 && t.end == 0 && t.next == 0
}

func (t *token32) isParentOf(u token32) bool {
	return t.begin <= u.begin && t.end >= u.end && t.next > u.next
}

func (t *token32) getToken32() token32 {
	return token32{pegRule: t.pegRule, begin: uint32(t.begin), end: uint32(t.end), next: uint32(t.next)}
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v %v", rul3s[t.pegRule], t.begin, t.end, t.next)
}

type tokens32 struct {
	tree    []token32
	ordered [][]token32
}

func (t *tokens32) trim(length int) {
	t.tree = t.tree[0:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) Order() [][]token32 {
	if t.ordered != nil {
		return t.ordered
	}

	depths := make([]int32, 1, math.MaxInt16)
	for i, token := range t.tree {
		if token.pegRule == ruleUnknown {
			t.tree = t.tree[:i]
			break
		}
		depth := int(token.next)
		if length := len(depths); depth >= length {
			depths = depths[:depth+1]
		}
		depths[depth]++
	}
	depths = append(depths, 0)

	ordered, pool := make([][]token32, len(depths)), make([]token32, len(t.tree)+len(depths))
	for i, depth := range depths {
		depth++
		ordered[i], pool, depths[i] = pool[:depth], pool[depth:], 0
	}

	for i, token := range t.tree {
		depth := token.next
		token.next = uint32(i)
		ordered[depth][depths[depth]] = token
		depths[depth]++
	}
	t.ordered = ordered
	return ordered
}

type state32 struct {
	token32
	depths []int32
	leaf   bool
}

func (t *tokens32) AST() *node32 {
	tokens := t.Tokens()
	stack := &element{node: &node32{token32: <-tokens}}
	for token := range tokens {
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
	return stack.node
}

func (t *tokens32) PreOrder() (<-chan state32, [][]token32) {
	s, ordered := make(chan state32, 6), t.Order()
	go func() {
		var states [8]state32
		for i, _ := range states {
			states[i].depths = make([]int32, len(ordered))
		}
		depths, state, depth := make([]int32, len(ordered)), 0, 1
		write := func(t token32, leaf bool) {
			S := states[state]
			state, S.pegRule, S.begin, S.end, S.next, S.leaf = (state+1)%8, t.pegRule, t.begin, t.end, uint32(depth), leaf
			copy(S.depths, depths)
			s <- S
		}

		states[state].token32 = ordered[0][0]
		depths[0]++
		state++
		a, b := ordered[depth-1][depths[depth-1]-1], ordered[depth][depths[depth]]
	depthFirstSearch:
		for {
			for {
				if i := depths[depth]; i > 0 {
					if c, j := ordered[depth][i-1], depths[depth-1]; a.isParentOf(c) &&
						(j < 2 || !ordered[depth-1][j-2].isParentOf(c)) {
						if c.end != b.begin {
							write(token32{pegRule: rule_In_, begin: c.end, end: b.begin}, true)
						}
						break
					}
				}

				if a.begin < b.begin {
					write(token32{pegRule: rulePre_, begin: a.begin, end: b.begin}, true)
				}
				break
			}

			next := depth + 1
			if c := ordered[next][depths[next]]; c.pegRule != ruleUnknown && b.isParentOf(c) {
				write(b, false)
				depths[depth]++
				depth, a, b = next, b, c
				continue
			}

			write(b, true)
			depths[depth]++
			c, parent := ordered[depth][depths[depth]], true
			for {
				if c.pegRule != ruleUnknown && a.isParentOf(c) {
					b = c
					continue depthFirstSearch
				} else if parent && b.end != a.end {
					write(token32{pegRule: rule_Suf, begin: b.end, end: a.end}, true)
				}

				depth--
				if depth > 0 {
					a, b, c = ordered[depth-1][depths[depth-1]-1], a, ordered[depth][depths[depth]]
					parent = a.isParentOf(b)
					continue
				}

				break depthFirstSearch
			}
		}

		close(s)
	}()
	return s, ordered
}

func (t *tokens32) PrintSyntax() {
	tokens, ordered := t.PreOrder()
	max := -1
	for token := range tokens {
		if !token.leaf {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[36m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[36m%v\x1B[m\n", rul3s[token.pegRule])
		} else if token.begin == token.end {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[31m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[31m%v\x1B[m\n", rul3s[token.pegRule])
		} else {
			for c, end := token.begin, token.end; c < end; c++ {
				if i := int(c); max+1 < i {
					for j := max; j < i; j++ {
						fmt.Printf("skip %v %v\n", j, token.String())
					}
					max = i
				} else if i := int(c); i <= max {
					for j := i; j <= max; j++ {
						fmt.Printf("dupe %v %v\n", j, token.String())
					}
				} else {
					max = int(c)
				}
				fmt.Printf("%v", c)
				for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
					fmt.Printf(" \x1B[34m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
				}
				fmt.Printf(" \x1B[34m%v\x1B[m\n", rul3s[token.pegRule])
			}
			fmt.Printf("\n")
		}
	}
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	tokens, _ := t.PreOrder()
	for token := range tokens {
		for c := 0; c < int(token.next); c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[token.pegRule], strconv.Quote(string(([]rune(buffer)[token.begin:token.end]))))
	}
}

func (t *tokens32) Add(rule pegRule, begin, end, depth uint32, index int) {
	t.tree[index] = token32{pegRule: rule, begin: uint32(begin), end: uint32(end), next: uint32(depth)}
}

func (t *tokens32) Tokens() <-chan token32 {
	s := make(chan token32, 16)
	go func() {
		for _, v := range t.tree {
			s <- v.getToken32()
		}
		close(s)
	}()
	return s
}

func (t *tokens32) Error() []token32 {
	ordered := t.Order()
	length := len(ordered)
	tokens, length := make([]token32, length), length-1
	for i, _ := range tokens {
		o := ordered[length-i]
		if len(o) > 1 {
			tokens[i] = o[len(o)-2].getToken32()
		}
	}
	return tokens
}

/*func (t *tokens16) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2 * len(tree))
		for i, v := range tree {
			expanded[i] = v.getToken32()
		}
		return &tokens32{tree: expanded}
	}
	return nil
}*/

func (t *tokens32) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	return nil
}

type Parser struct {
	tables []Table
	table  *Table
	column *Column

	Buffer string
	buffer []rune
	rules  [34]func() bool
	Parse  func(rule ...int) error
	Reset  func()
	Pretty bool
	tokenTree
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
	p.tokenTree.PrintSyntaxTree(p.Buffer)
}

func (p *Parser) Highlighter() {
	p.tokenTree.PrintSyntax()
}

func (p *Parser) Execute() {
	buffer, _buffer, text, begin, end := p.Buffer, p.buffer, "", 0, 0
	for token := range p.tokenTree.Tokens() {
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
	p.buffer = []rune(p.Buffer)
	if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != end_symbol {
		p.buffer = append(p.buffer, end_symbol)
	}

	var tree tokenTree = &tokens32{tree: make([]token32, math.MaxInt16)}
	var max token32
	position, depth, tokenIndex, buffer, _rules := uint32(0), uint32(0), 0, p.buffer, p.rules

	p.Parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokenTree = tree
		if matches {
			p.tokenTree.trim(tokenIndex)
			return nil
		}
		return &parseError{p, max}
	}

	p.Reset = func() {
		position, tokenIndex, depth = 0, 0, 0
	}

	add := func(rule pegRule, begin uint32) {
		if t := tree.Expand(tokenIndex); t != nil {
			tree = t
		}
		tree.Add(rule, begin, position, depth, tokenIndex)
		tokenIndex++
		if begin != position && position > max.end {
			max = token32{rule, begin, position, depth}
		}
	}

	matchDot := func() bool {
		if buffer[position] != end_symbol {
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
			position0, tokenIndex0, depth0 := position, tokenIndex, depth
			{
				position1 := position
				depth++
			l2:
				{
					position3, tokenIndex3, depth3 := position, tokenIndex, depth
				l4:
					{
						position5, tokenIndex5, depth5 := position, tokenIndex, depth
						if !_rules[ruleSep]() {
							goto l5
						}
						goto l4
					l5:
						position, tokenIndex, depth = position5, tokenIndex5, depth5
					}
					if !_rules[ruleTableDef]() {
						goto l3
					}
					goto l2
				l3:
					position, tokenIndex, depth = position3, tokenIndex3, depth3
				}
			l6:
				{
					position7, tokenIndex7, depth7 := position, tokenIndex, depth
					if !_rules[ruleSep]() {
						goto l7
					}
					goto l6
				l7:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
				}
				if !_rules[ruleEOT]() {
					goto l0
				}
				depth--
				add(ruleroot, position1)
			}
			return true
		l0:
			position, tokenIndex, depth = position0, tokenIndex0, depth0
			return false
		},
		/* 1 Sep <- <('\n' / '\t' / ' ')+> */
		func() bool {
			position8, tokenIndex8, depth8 := position, tokenIndex, depth
			{
				position9 := position
				depth++
				{
					position12, tokenIndex12, depth12 := position, tokenIndex, depth
					if buffer[position] != rune('\n') {
						goto l13
					}
					position++
					goto l12
				l13:
					position, tokenIndex, depth = position12, tokenIndex12, depth12
					if buffer[position] != rune('\t') {
						goto l14
					}
					position++
					goto l12
				l14:
					position, tokenIndex, depth = position12, tokenIndex12, depth12
					if buffer[position] != rune(' ') {
						goto l8
					}
					position++
				}
			l12:
			l10:
				{
					position11, tokenIndex11, depth11 := position, tokenIndex, depth
					{
						position15, tokenIndex15, depth15 := position, tokenIndex, depth
						if buffer[position] != rune('\n') {
							goto l16
						}
						position++
						goto l15
					l16:
						position, tokenIndex, depth = position15, tokenIndex15, depth15
						if buffer[position] != rune('\t') {
							goto l17
						}
						position++
						goto l15
					l17:
						position, tokenIndex, depth = position15, tokenIndex15, depth15
						if buffer[position] != rune(' ') {
							goto l11
						}
						position++
					}
				l15:
					goto l10
				l11:
					position, tokenIndex, depth = position11, tokenIndex11, depth11
				}
				depth--
				add(ruleSep, position9)
			}
			return true
		l8:
			position, tokenIndex, depth = position8, tokenIndex8, depth8
			return false
		},
		/* 2 Space <- <' '> */
		func() bool {
			position18, tokenIndex18, depth18 := position, tokenIndex, depth
			{
				position19 := position
				depth++
				if buffer[position] != rune(' ') {
					goto l18
				}
				position++
				depth--
				add(ruleSpace, position19)
			}
			return true
		l18:
			position, tokenIndex, depth = position18, tokenIndex18, depth18
			return false
		},
		/* 3 TableDef <- <(TableName Sep (':' Space* TableDescription)? LeftBrace Sep Columns Sep RightBrace)> */
		func() bool {
			position20, tokenIndex20, depth20 := position, tokenIndex, depth
			{
				position21 := position
				depth++
				if !_rules[ruleTableName]() {
					goto l20
				}
				if !_rules[ruleSep]() {
					goto l20
				}
				{
					position22, tokenIndex22, depth22 := position, tokenIndex, depth
					if buffer[position] != rune(':') {
						goto l22
					}
					position++
				l24:
					{
						position25, tokenIndex25, depth25 := position, tokenIndex, depth
						if !_rules[ruleSpace]() {
							goto l25
						}
						goto l24
					l25:
						position, tokenIndex, depth = position25, tokenIndex25, depth25
					}
					if !_rules[ruleTableDescription]() {
						goto l22
					}
					goto l23
				l22:
					position, tokenIndex, depth = position22, tokenIndex22, depth22
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
				depth--
				add(ruleTableDef, position21)
			}
			return true
		l20:
			position, tokenIndex, depth = position20, tokenIndex20, depth20
			return false
		},
		/* 4 LeftBrace <- <'{'> */
		func() bool {
			position26, tokenIndex26, depth26 := position, tokenIndex, depth
			{
				position27 := position
				depth++
				if buffer[position] != rune('{') {
					goto l26
				}
				position++
				depth--
				add(ruleLeftBrace, position27)
			}
			return true
		l26:
			position, tokenIndex, depth = position26, tokenIndex26, depth26
			return false
		},
		/* 5 RightBrace <- <('}' Action0)> */
		func() bool {
			position28, tokenIndex28, depth28 := position, tokenIndex, depth
			{
				position29 := position
				depth++
				if buffer[position] != rune('}') {
					goto l28
				}
				position++
				if !_rules[ruleAction0]() {
					goto l28
				}
				depth--
				add(ruleRightBrace, position29)
			}
			return true
		l28:
			position, tokenIndex, depth = position28, tokenIndex28, depth28
			return false
		},
		/* 6 TableName <- <(<([a-z] / [A-Z] / '_')+> Action1)> */
		func() bool {
			position30, tokenIndex30, depth30 := position, tokenIndex, depth
			{
				position31 := position
				depth++
				{
					position32 := position
					depth++
					{
						position35, tokenIndex35, depth35 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l36
						}
						position++
						goto l35
					l36:
						position, tokenIndex, depth = position35, tokenIndex35, depth35
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l37
						}
						position++
						goto l35
					l37:
						position, tokenIndex, depth = position35, tokenIndex35, depth35
						if buffer[position] != rune('_') {
							goto l30
						}
						position++
					}
				l35:
				l33:
					{
						position34, tokenIndex34, depth34 := position, tokenIndex, depth
						{
							position38, tokenIndex38, depth38 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l39
							}
							position++
							goto l38
						l39:
							position, tokenIndex, depth = position38, tokenIndex38, depth38
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l40
							}
							position++
							goto l38
						l40:
							position, tokenIndex, depth = position38, tokenIndex38, depth38
							if buffer[position] != rune('_') {
								goto l34
							}
							position++
						}
					l38:
						goto l33
					l34:
						position, tokenIndex, depth = position34, tokenIndex34, depth34
					}
					depth--
					add(rulePegText, position32)
				}
				if !_rules[ruleAction1]() {
					goto l30
				}
				depth--
				add(ruleTableName, position31)
			}
			return true
		l30:
			position, tokenIndex, depth = position30, tokenIndex30, depth30
			return false
		},
		/* 7 TableDescription <- <(<(!('\n' / '{') .)+> Action2)> */
		func() bool {
			position41, tokenIndex41, depth41 := position, tokenIndex, depth
			{
				position42 := position
				depth++
				{
					position43 := position
					depth++
					{
						position46, tokenIndex46, depth46 := position, tokenIndex, depth
						{
							position47, tokenIndex47, depth47 := position, tokenIndex, depth
							if buffer[position] != rune('\n') {
								goto l48
							}
							position++
							goto l47
						l48:
							position, tokenIndex, depth = position47, tokenIndex47, depth47
							if buffer[position] != rune('{') {
								goto l46
							}
							position++
						}
					l47:
						goto l41
					l46:
						position, tokenIndex, depth = position46, tokenIndex46, depth46
					}
					if !matchDot() {
						goto l41
					}
				l44:
					{
						position45, tokenIndex45, depth45 := position, tokenIndex, depth
						{
							position49, tokenIndex49, depth49 := position, tokenIndex, depth
							{
								position50, tokenIndex50, depth50 := position, tokenIndex, depth
								if buffer[position] != rune('\n') {
									goto l51
								}
								position++
								goto l50
							l51:
								position, tokenIndex, depth = position50, tokenIndex50, depth50
								if buffer[position] != rune('{') {
									goto l49
								}
								position++
							}
						l50:
							goto l45
						l49:
							position, tokenIndex, depth = position49, tokenIndex49, depth49
						}
						if !matchDot() {
							goto l45
						}
						goto l44
					l45:
						position, tokenIndex, depth = position45, tokenIndex45, depth45
					}
					depth--
					add(rulePegText, position43)
				}
				if !_rules[ruleAction2]() {
					goto l41
				}
				depth--
				add(ruleTableDescription, position42)
			}
			return true
		l41:
			position, tokenIndex, depth = position41, tokenIndex41, depth41
			return false
		},
		/* 8 Columns <- <(Column (Sep Column)*)> */
		func() bool {
			position52, tokenIndex52, depth52 := position, tokenIndex, depth
			{
				position53 := position
				depth++
				if !_rules[ruleColumn]() {
					goto l52
				}
			l54:
				{
					position55, tokenIndex55, depth55 := position, tokenIndex, depth
					if !_rules[ruleSep]() {
						goto l55
					}
					if !_rules[ruleColumn]() {
						goto l55
					}
					goto l54
				l55:
					position, tokenIndex, depth = position55, tokenIndex55, depth55
				}
				depth--
				add(ruleColumns, position53)
			}
			return true
		l52:
			position, tokenIndex, depth = position52, tokenIndex52, depth52
			return false
		},
		/* 9 Column <- <(ColumnDef Space* (RightArrow Sep TargetTableName dot TargetColumnName Space*)? (':' Space* ColumnDescription)? Action3)> */
		func() bool {
			position56, tokenIndex56, depth56 := position, tokenIndex, depth
			{
				position57 := position
				depth++
				if !_rules[ruleColumnDef]() {
					goto l56
				}
			l58:
				{
					position59, tokenIndex59, depth59 := position, tokenIndex, depth
					if !_rules[ruleSpace]() {
						goto l59
					}
					goto l58
				l59:
					position, tokenIndex, depth = position59, tokenIndex59, depth59
				}
				{
					position60, tokenIndex60, depth60 := position, tokenIndex, depth
					if !_rules[ruleRightArrow]() {
						goto l60
					}
					if !_rules[ruleSep]() {
						goto l60
					}
					if !_rules[ruleTargetTableName]() {
						goto l60
					}
					if !_rules[ruledot]() {
						goto l60
					}
					if !_rules[ruleTargetColumnName]() {
						goto l60
					}
				l62:
					{
						position63, tokenIndex63, depth63 := position, tokenIndex, depth
						if !_rules[ruleSpace]() {
							goto l63
						}
						goto l62
					l63:
						position, tokenIndex, depth = position63, tokenIndex63, depth63
					}
					goto l61
				l60:
					position, tokenIndex, depth = position60, tokenIndex60, depth60
				}
			l61:
				{
					position64, tokenIndex64, depth64 := position, tokenIndex, depth
					if buffer[position] != rune(':') {
						goto l64
					}
					position++
				l66:
					{
						position67, tokenIndex67, depth67 := position, tokenIndex, depth
						if !_rules[ruleSpace]() {
							goto l67
						}
						goto l66
					l67:
						position, tokenIndex, depth = position67, tokenIndex67, depth67
					}
					if !_rules[ruleColumnDescription]() {
						goto l64
					}
					goto l65
				l64:
					position, tokenIndex, depth = position64, tokenIndex64, depth64
				}
			l65:
				if !_rules[ruleAction3]() {
					goto l56
				}
				depth--
				add(ruleColumn, position57)
			}
			return true
		l56:
			position, tokenIndex, depth = position56, tokenIndex56, depth56
			return false
		},
		/* 10 ColumnDescription <- <(<(!'\n' .)+> Action4)> */
		func() bool {
			position68, tokenIndex68, depth68 := position, tokenIndex, depth
			{
				position69 := position
				depth++
				{
					position70 := position
					depth++
					{
						position73, tokenIndex73, depth73 := position, tokenIndex, depth
						if buffer[position] != rune('\n') {
							goto l73
						}
						position++
						goto l68
					l73:
						position, tokenIndex, depth = position73, tokenIndex73, depth73
					}
					if !matchDot() {
						goto l68
					}
				l71:
					{
						position72, tokenIndex72, depth72 := position, tokenIndex, depth
						{
							position74, tokenIndex74, depth74 := position, tokenIndex, depth
							if buffer[position] != rune('\n') {
								goto l74
							}
							position++
							goto l72
						l74:
							position, tokenIndex, depth = position74, tokenIndex74, depth74
						}
						if !matchDot() {
							goto l72
						}
						goto l71
					l72:
						position, tokenIndex, depth = position72, tokenIndex72, depth72
					}
					depth--
					add(rulePegText, position70)
				}
				if !_rules[ruleAction4]() {
					goto l68
				}
				depth--
				add(ruleColumnDescription, position69)
			}
			return true
		l68:
			position, tokenIndex, depth = position68, tokenIndex68, depth68
			return false
		},
		/* 11 dot <- <'.'> */
		func() bool {
			position75, tokenIndex75, depth75 := position, tokenIndex, depth
			{
				position76 := position
				depth++
				if buffer[position] != rune('.') {
					goto l75
				}
				position++
				depth--
				add(ruledot, position76)
			}
			return true
		l75:
			position, tokenIndex, depth = position75, tokenIndex75, depth75
			return false
		},
		/* 12 ColumnName <- <(<([a-z] / [A-Z] / '_')+> Action5)> */
		func() bool {
			position77, tokenIndex77, depth77 := position, tokenIndex, depth
			{
				position78 := position
				depth++
				{
					position79 := position
					depth++
					{
						position82, tokenIndex82, depth82 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l83
						}
						position++
						goto l82
					l83:
						position, tokenIndex, depth = position82, tokenIndex82, depth82
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l84
						}
						position++
						goto l82
					l84:
						position, tokenIndex, depth = position82, tokenIndex82, depth82
						if buffer[position] != rune('_') {
							goto l77
						}
						position++
					}
				l82:
				l80:
					{
						position81, tokenIndex81, depth81 := position, tokenIndex, depth
						{
							position85, tokenIndex85, depth85 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l86
							}
							position++
							goto l85
						l86:
							position, tokenIndex, depth = position85, tokenIndex85, depth85
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l87
							}
							position++
							goto l85
						l87:
							position, tokenIndex, depth = position85, tokenIndex85, depth85
							if buffer[position] != rune('_') {
								goto l81
							}
							position++
						}
					l85:
						goto l80
					l81:
						position, tokenIndex, depth = position81, tokenIndex81, depth81
					}
					depth--
					add(rulePegText, position79)
				}
				if !_rules[ruleAction5]() {
					goto l77
				}
				depth--
				add(ruleColumnName, position78)
			}
			return true
		l77:
			position, tokenIndex, depth = position77, tokenIndex77, depth77
			return false
		},
		/* 13 ColumnDef <- <(ColumnName (Space* ColumnType)?)> */
		func() bool {
			position88, tokenIndex88, depth88 := position, tokenIndex, depth
			{
				position89 := position
				depth++
				if !_rules[ruleColumnName]() {
					goto l88
				}
				{
					position90, tokenIndex90, depth90 := position, tokenIndex, depth
				l92:
					{
						position93, tokenIndex93, depth93 := position, tokenIndex, depth
						if !_rules[ruleSpace]() {
							goto l93
						}
						goto l92
					l93:
						position, tokenIndex, depth = position93, tokenIndex93, depth93
					}
					if !_rules[ruleColumnType]() {
						goto l90
					}
					goto l91
				l90:
					position, tokenIndex, depth = position90, tokenIndex90, depth90
				}
			l91:
				depth--
				add(ruleColumnDef, position89)
			}
			return true
		l88:
			position, tokenIndex, depth = position88, tokenIndex88, depth88
			return false
		},
		/* 14 RightArrow <- <(RightDotArrow / RightLineArrow)> */
		func() bool {
			position94, tokenIndex94, depth94 := position, tokenIndex, depth
			{
				position95 := position
				depth++
				{
					position96, tokenIndex96, depth96 := position, tokenIndex, depth
					if !_rules[ruleRightDotArrow]() {
						goto l97
					}
					goto l96
				l97:
					position, tokenIndex, depth = position96, tokenIndex96, depth96
					if !_rules[ruleRightLineArrow]() {
						goto l94
					}
				}
			l96:
				depth--
				add(ruleRightArrow, position95)
			}
			return true
		l94:
			position, tokenIndex, depth = position94, tokenIndex94, depth94
			return false
		},
		/* 15 ColumnType <- <(<(!('-' / ':' / '.' / '\n') .)+> Action6)> */
		func() bool {
			position98, tokenIndex98, depth98 := position, tokenIndex, depth
			{
				position99 := position
				depth++
				{
					position100 := position
					depth++
					{
						position103, tokenIndex103, depth103 := position, tokenIndex, depth
						{
							position104, tokenIndex104, depth104 := position, tokenIndex, depth
							if buffer[position] != rune('-') {
								goto l105
							}
							position++
							goto l104
						l105:
							position, tokenIndex, depth = position104, tokenIndex104, depth104
							if buffer[position] != rune(':') {
								goto l106
							}
							position++
							goto l104
						l106:
							position, tokenIndex, depth = position104, tokenIndex104, depth104
							if buffer[position] != rune('.') {
								goto l107
							}
							position++
							goto l104
						l107:
							position, tokenIndex, depth = position104, tokenIndex104, depth104
							if buffer[position] != rune('\n') {
								goto l103
							}
							position++
						}
					l104:
						goto l98
					l103:
						position, tokenIndex, depth = position103, tokenIndex103, depth103
					}
					if !matchDot() {
						goto l98
					}
				l101:
					{
						position102, tokenIndex102, depth102 := position, tokenIndex, depth
						{
							position108, tokenIndex108, depth108 := position, tokenIndex, depth
							{
								position109, tokenIndex109, depth109 := position, tokenIndex, depth
								if buffer[position] != rune('-') {
									goto l110
								}
								position++
								goto l109
							l110:
								position, tokenIndex, depth = position109, tokenIndex109, depth109
								if buffer[position] != rune(':') {
									goto l111
								}
								position++
								goto l109
							l111:
								position, tokenIndex, depth = position109, tokenIndex109, depth109
								if buffer[position] != rune('.') {
									goto l112
								}
								position++
								goto l109
							l112:
								position, tokenIndex, depth = position109, tokenIndex109, depth109
								if buffer[position] != rune('\n') {
									goto l108
								}
								position++
							}
						l109:
							goto l102
						l108:
							position, tokenIndex, depth = position108, tokenIndex108, depth108
						}
						if !matchDot() {
							goto l102
						}
						goto l101
					l102:
						position, tokenIndex, depth = position102, tokenIndex102, depth102
					}
					depth--
					add(rulePegText, position100)
				}
				if !_rules[ruleAction6]() {
					goto l98
				}
				depth--
				add(ruleColumnType, position99)
			}
			return true
		l98:
			position, tokenIndex, depth = position98, tokenIndex98, depth98
			return false
		},
		/* 16 RightDotArrow <- <('.' '.' '>' Action7)> */
		func() bool {
			position113, tokenIndex113, depth113 := position, tokenIndex, depth
			{
				position114 := position
				depth++
				if buffer[position] != rune('.') {
					goto l113
				}
				position++
				if buffer[position] != rune('.') {
					goto l113
				}
				position++
				if buffer[position] != rune('>') {
					goto l113
				}
				position++
				if !_rules[ruleAction7]() {
					goto l113
				}
				depth--
				add(ruleRightDotArrow, position114)
			}
			return true
		l113:
			position, tokenIndex, depth = position113, tokenIndex113, depth113
			return false
		},
		/* 17 RightLineArrow <- <('-' '>' Action8)> */
		func() bool {
			position115, tokenIndex115, depth115 := position, tokenIndex, depth
			{
				position116 := position
				depth++
				if buffer[position] != rune('-') {
					goto l115
				}
				position++
				if buffer[position] != rune('>') {
					goto l115
				}
				position++
				if !_rules[ruleAction8]() {
					goto l115
				}
				depth--
				add(ruleRightLineArrow, position116)
			}
			return true
		l115:
			position, tokenIndex, depth = position115, tokenIndex115, depth115
			return false
		},
		/* 18 TargetTableName <- <(<([a-z] / [A-Z] / '_')+> Action9)> */
		func() bool {
			position117, tokenIndex117, depth117 := position, tokenIndex, depth
			{
				position118 := position
				depth++
				{
					position119 := position
					depth++
					{
						position122, tokenIndex122, depth122 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l123
						}
						position++
						goto l122
					l123:
						position, tokenIndex, depth = position122, tokenIndex122, depth122
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l124
						}
						position++
						goto l122
					l124:
						position, tokenIndex, depth = position122, tokenIndex122, depth122
						if buffer[position] != rune('_') {
							goto l117
						}
						position++
					}
				l122:
				l120:
					{
						position121, tokenIndex121, depth121 := position, tokenIndex, depth
						{
							position125, tokenIndex125, depth125 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l126
							}
							position++
							goto l125
						l126:
							position, tokenIndex, depth = position125, tokenIndex125, depth125
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l127
							}
							position++
							goto l125
						l127:
							position, tokenIndex, depth = position125, tokenIndex125, depth125
							if buffer[position] != rune('_') {
								goto l121
							}
							position++
						}
					l125:
						goto l120
					l121:
						position, tokenIndex, depth = position121, tokenIndex121, depth121
					}
					depth--
					add(rulePegText, position119)
				}
				if !_rules[ruleAction9]() {
					goto l117
				}
				depth--
				add(ruleTargetTableName, position118)
			}
			return true
		l117:
			position, tokenIndex, depth = position117, tokenIndex117, depth117
			return false
		},
		/* 19 TargetColumnName <- <(<([a-z] / [A-Z] / '_')+> Action10)> */
		func() bool {
			position128, tokenIndex128, depth128 := position, tokenIndex, depth
			{
				position129 := position
				depth++
				{
					position130 := position
					depth++
					{
						position133, tokenIndex133, depth133 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l134
						}
						position++
						goto l133
					l134:
						position, tokenIndex, depth = position133, tokenIndex133, depth133
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l135
						}
						position++
						goto l133
					l135:
						position, tokenIndex, depth = position133, tokenIndex133, depth133
						if buffer[position] != rune('_') {
							goto l128
						}
						position++
					}
				l133:
				l131:
					{
						position132, tokenIndex132, depth132 := position, tokenIndex, depth
						{
							position136, tokenIndex136, depth136 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l137
							}
							position++
							goto l136
						l137:
							position, tokenIndex, depth = position136, tokenIndex136, depth136
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l138
							}
							position++
							goto l136
						l138:
							position, tokenIndex, depth = position136, tokenIndex136, depth136
							if buffer[position] != rune('_') {
								goto l132
							}
							position++
						}
					l136:
						goto l131
					l132:
						position, tokenIndex, depth = position132, tokenIndex132, depth132
					}
					depth--
					add(rulePegText, position130)
				}
				if !_rules[ruleAction10]() {
					goto l128
				}
				depth--
				add(ruleTargetColumnName, position129)
			}
			return true
		l128:
			position, tokenIndex, depth = position128, tokenIndex128, depth128
			return false
		},
		/* 20 EOT <- <!.> */
		func() bool {
			position139, tokenIndex139, depth139 := position, tokenIndex, depth
			{
				position140 := position
				depth++
				{
					position141, tokenIndex141, depth141 := position, tokenIndex, depth
					if !matchDot() {
						goto l141
					}
					goto l139
				l141:
					position, tokenIndex, depth = position141, tokenIndex141, depth141
				}
				depth--
				add(ruleEOT, position140)
			}
			return true
		l139:
			position, tokenIndex, depth = position139, tokenIndex139, depth139
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
