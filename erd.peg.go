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

	rulePre_
	rule_In_
	rule_Suf
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
	rules  [35]func() bool
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
		/* 1 Comment <- <('\n' '#' (!'\n' .)+ '\n')> */
		func() bool {
			position8, tokenIndex8, depth8 := position, tokenIndex, depth
			{
				position9 := position
				depth++
				if buffer[position] != rune('\n') {
					goto l8
				}
				position++
				if buffer[position] != rune('#') {
					goto l8
				}
				position++
				{
					position12, tokenIndex12, depth12 := position, tokenIndex, depth
					if buffer[position] != rune('\n') {
						goto l12
					}
					position++
					goto l8
				l12:
					position, tokenIndex, depth = position12, tokenIndex12, depth12
				}
				if !matchDot() {
					goto l8
				}
			l10:
				{
					position11, tokenIndex11, depth11 := position, tokenIndex, depth
					{
						position13, tokenIndex13, depth13 := position, tokenIndex, depth
						if buffer[position] != rune('\n') {
							goto l13
						}
						position++
						goto l11
					l13:
						position, tokenIndex, depth = position13, tokenIndex13, depth13
					}
					if !matchDot() {
						goto l11
					}
					goto l10
				l11:
					position, tokenIndex, depth = position11, tokenIndex11, depth11
				}
				if buffer[position] != rune('\n') {
					goto l8
				}
				position++
				depth--
				add(ruleComment, position9)
			}
			return true
		l8:
			position, tokenIndex, depth = position8, tokenIndex8, depth8
			return false
		},
		/* 2 Sep <- <(Comment / ('\n' / '\t' / ' '))> */
		func() bool {
			position14, tokenIndex14, depth14 := position, tokenIndex, depth
			{
				position15 := position
				depth++
				{
					position16, tokenIndex16, depth16 := position, tokenIndex, depth
					if !_rules[ruleComment]() {
						goto l17
					}
					goto l16
				l17:
					position, tokenIndex, depth = position16, tokenIndex16, depth16
					{
						position18, tokenIndex18, depth18 := position, tokenIndex, depth
						if buffer[position] != rune('\n') {
							goto l19
						}
						position++
						goto l18
					l19:
						position, tokenIndex, depth = position18, tokenIndex18, depth18
						if buffer[position] != rune('\t') {
							goto l20
						}
						position++
						goto l18
					l20:
						position, tokenIndex, depth = position18, tokenIndex18, depth18
						if buffer[position] != rune(' ') {
							goto l14
						}
						position++
					}
				l18:
				}
			l16:
				depth--
				add(ruleSep, position15)
			}
			return true
		l14:
			position, tokenIndex, depth = position14, tokenIndex14, depth14
			return false
		},
		/* 3 Space <- <' '> */
		func() bool {
			position21, tokenIndex21, depth21 := position, tokenIndex, depth
			{
				position22 := position
				depth++
				if buffer[position] != rune(' ') {
					goto l21
				}
				position++
				depth--
				add(ruleSpace, position22)
			}
			return true
		l21:
			position, tokenIndex, depth = position21, tokenIndex21, depth21
			return false
		},
		/* 4 TableDef <- <(TableName Sep+ (':' Space* TableDescription)? LeftBrace Sep+ Columns Sep+ RightBrace)> */
		func() bool {
			position23, tokenIndex23, depth23 := position, tokenIndex, depth
			{
				position24 := position
				depth++
				if !_rules[ruleTableName]() {
					goto l23
				}
				if !_rules[ruleSep]() {
					goto l23
				}
			l25:
				{
					position26, tokenIndex26, depth26 := position, tokenIndex, depth
					if !_rules[ruleSep]() {
						goto l26
					}
					goto l25
				l26:
					position, tokenIndex, depth = position26, tokenIndex26, depth26
				}
				{
					position27, tokenIndex27, depth27 := position, tokenIndex, depth
					if buffer[position] != rune(':') {
						goto l27
					}
					position++
				l29:
					{
						position30, tokenIndex30, depth30 := position, tokenIndex, depth
						if !_rules[ruleSpace]() {
							goto l30
						}
						goto l29
					l30:
						position, tokenIndex, depth = position30, tokenIndex30, depth30
					}
					if !_rules[ruleTableDescription]() {
						goto l27
					}
					goto l28
				l27:
					position, tokenIndex, depth = position27, tokenIndex27, depth27
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
					position32, tokenIndex32, depth32 := position, tokenIndex, depth
					if !_rules[ruleSep]() {
						goto l32
					}
					goto l31
				l32:
					position, tokenIndex, depth = position32, tokenIndex32, depth32
				}
				if !_rules[ruleColumns]() {
					goto l23
				}
				if !_rules[ruleSep]() {
					goto l23
				}
			l33:
				{
					position34, tokenIndex34, depth34 := position, tokenIndex, depth
					if !_rules[ruleSep]() {
						goto l34
					}
					goto l33
				l34:
					position, tokenIndex, depth = position34, tokenIndex34, depth34
				}
				if !_rules[ruleRightBrace]() {
					goto l23
				}
				depth--
				add(ruleTableDef, position24)
			}
			return true
		l23:
			position, tokenIndex, depth = position23, tokenIndex23, depth23
			return false
		},
		/* 5 LeftBrace <- <'{'> */
		func() bool {
			position35, tokenIndex35, depth35 := position, tokenIndex, depth
			{
				position36 := position
				depth++
				if buffer[position] != rune('{') {
					goto l35
				}
				position++
				depth--
				add(ruleLeftBrace, position36)
			}
			return true
		l35:
			position, tokenIndex, depth = position35, tokenIndex35, depth35
			return false
		},
		/* 6 RightBrace <- <('}' Action0)> */
		func() bool {
			position37, tokenIndex37, depth37 := position, tokenIndex, depth
			{
				position38 := position
				depth++
				if buffer[position] != rune('}') {
					goto l37
				}
				position++
				if !_rules[ruleAction0]() {
					goto l37
				}
				depth--
				add(ruleRightBrace, position38)
			}
			return true
		l37:
			position, tokenIndex, depth = position37, tokenIndex37, depth37
			return false
		},
		/* 7 TableName <- <(<([a-z] / [A-Z] / '_')+> Action1)> */
		func() bool {
			position39, tokenIndex39, depth39 := position, tokenIndex, depth
			{
				position40 := position
				depth++
				{
					position41 := position
					depth++
					{
						position44, tokenIndex44, depth44 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l45
						}
						position++
						goto l44
					l45:
						position, tokenIndex, depth = position44, tokenIndex44, depth44
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l46
						}
						position++
						goto l44
					l46:
						position, tokenIndex, depth = position44, tokenIndex44, depth44
						if buffer[position] != rune('_') {
							goto l39
						}
						position++
					}
				l44:
				l42:
					{
						position43, tokenIndex43, depth43 := position, tokenIndex, depth
						{
							position47, tokenIndex47, depth47 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l48
							}
							position++
							goto l47
						l48:
							position, tokenIndex, depth = position47, tokenIndex47, depth47
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l49
							}
							position++
							goto l47
						l49:
							position, tokenIndex, depth = position47, tokenIndex47, depth47
							if buffer[position] != rune('_') {
								goto l43
							}
							position++
						}
					l47:
						goto l42
					l43:
						position, tokenIndex, depth = position43, tokenIndex43, depth43
					}
					depth--
					add(rulePegText, position41)
				}
				if !_rules[ruleAction1]() {
					goto l39
				}
				depth--
				add(ruleTableName, position40)
			}
			return true
		l39:
			position, tokenIndex, depth = position39, tokenIndex39, depth39
			return false
		},
		/* 8 TableDescription <- <(<(!('\n' / '{') .)+> Action2)> */
		func() bool {
			position50, tokenIndex50, depth50 := position, tokenIndex, depth
			{
				position51 := position
				depth++
				{
					position52 := position
					depth++
					{
						position55, tokenIndex55, depth55 := position, tokenIndex, depth
						{
							position56, tokenIndex56, depth56 := position, tokenIndex, depth
							if buffer[position] != rune('\n') {
								goto l57
							}
							position++
							goto l56
						l57:
							position, tokenIndex, depth = position56, tokenIndex56, depth56
							if buffer[position] != rune('{') {
								goto l55
							}
							position++
						}
					l56:
						goto l50
					l55:
						position, tokenIndex, depth = position55, tokenIndex55, depth55
					}
					if !matchDot() {
						goto l50
					}
				l53:
					{
						position54, tokenIndex54, depth54 := position, tokenIndex, depth
						{
							position58, tokenIndex58, depth58 := position, tokenIndex, depth
							{
								position59, tokenIndex59, depth59 := position, tokenIndex, depth
								if buffer[position] != rune('\n') {
									goto l60
								}
								position++
								goto l59
							l60:
								position, tokenIndex, depth = position59, tokenIndex59, depth59
								if buffer[position] != rune('{') {
									goto l58
								}
								position++
							}
						l59:
							goto l54
						l58:
							position, tokenIndex, depth = position58, tokenIndex58, depth58
						}
						if !matchDot() {
							goto l54
						}
						goto l53
					l54:
						position, tokenIndex, depth = position54, tokenIndex54, depth54
					}
					depth--
					add(rulePegText, position52)
				}
				if !_rules[ruleAction2]() {
					goto l50
				}
				depth--
				add(ruleTableDescription, position51)
			}
			return true
		l50:
			position, tokenIndex, depth = position50, tokenIndex50, depth50
			return false
		},
		/* 9 Columns <- <(Column (Sep+ Column)*)> */
		func() bool {
			position61, tokenIndex61, depth61 := position, tokenIndex, depth
			{
				position62 := position
				depth++
				if !_rules[ruleColumn]() {
					goto l61
				}
			l63:
				{
					position64, tokenIndex64, depth64 := position, tokenIndex, depth
					if !_rules[ruleSep]() {
						goto l64
					}
				l65:
					{
						position66, tokenIndex66, depth66 := position, tokenIndex, depth
						if !_rules[ruleSep]() {
							goto l66
						}
						goto l65
					l66:
						position, tokenIndex, depth = position66, tokenIndex66, depth66
					}
					if !_rules[ruleColumn]() {
						goto l64
					}
					goto l63
				l64:
					position, tokenIndex, depth = position64, tokenIndex64, depth64
				}
				depth--
				add(ruleColumns, position62)
			}
			return true
		l61:
			position, tokenIndex, depth = position61, tokenIndex61, depth61
			return false
		},
		/* 10 Column <- <(ColumnDef Space* (RightArrow Sep+ TargetTableName dot TargetColumnName Space*)? (':' Space* ColumnDescription)? Action3)> */
		func() bool {
			position67, tokenIndex67, depth67 := position, tokenIndex, depth
			{
				position68 := position
				depth++
				if !_rules[ruleColumnDef]() {
					goto l67
				}
			l69:
				{
					position70, tokenIndex70, depth70 := position, tokenIndex, depth
					if !_rules[ruleSpace]() {
						goto l70
					}
					goto l69
				l70:
					position, tokenIndex, depth = position70, tokenIndex70, depth70
				}
				{
					position71, tokenIndex71, depth71 := position, tokenIndex, depth
					if !_rules[ruleRightArrow]() {
						goto l71
					}
					if !_rules[ruleSep]() {
						goto l71
					}
				l73:
					{
						position74, tokenIndex74, depth74 := position, tokenIndex, depth
						if !_rules[ruleSep]() {
							goto l74
						}
						goto l73
					l74:
						position, tokenIndex, depth = position74, tokenIndex74, depth74
					}
					if !_rules[ruleTargetTableName]() {
						goto l71
					}
					if !_rules[ruledot]() {
						goto l71
					}
					if !_rules[ruleTargetColumnName]() {
						goto l71
					}
				l75:
					{
						position76, tokenIndex76, depth76 := position, tokenIndex, depth
						if !_rules[ruleSpace]() {
							goto l76
						}
						goto l75
					l76:
						position, tokenIndex, depth = position76, tokenIndex76, depth76
					}
					goto l72
				l71:
					position, tokenIndex, depth = position71, tokenIndex71, depth71
				}
			l72:
				{
					position77, tokenIndex77, depth77 := position, tokenIndex, depth
					if buffer[position] != rune(':') {
						goto l77
					}
					position++
				l79:
					{
						position80, tokenIndex80, depth80 := position, tokenIndex, depth
						if !_rules[ruleSpace]() {
							goto l80
						}
						goto l79
					l80:
						position, tokenIndex, depth = position80, tokenIndex80, depth80
					}
					if !_rules[ruleColumnDescription]() {
						goto l77
					}
					goto l78
				l77:
					position, tokenIndex, depth = position77, tokenIndex77, depth77
				}
			l78:
				if !_rules[ruleAction3]() {
					goto l67
				}
				depth--
				add(ruleColumn, position68)
			}
			return true
		l67:
			position, tokenIndex, depth = position67, tokenIndex67, depth67
			return false
		},
		/* 11 ColumnDescription <- <(<(!'\n' .)+> Action4)> */
		func() bool {
			position81, tokenIndex81, depth81 := position, tokenIndex, depth
			{
				position82 := position
				depth++
				{
					position83 := position
					depth++
					{
						position86, tokenIndex86, depth86 := position, tokenIndex, depth
						if buffer[position] != rune('\n') {
							goto l86
						}
						position++
						goto l81
					l86:
						position, tokenIndex, depth = position86, tokenIndex86, depth86
					}
					if !matchDot() {
						goto l81
					}
				l84:
					{
						position85, tokenIndex85, depth85 := position, tokenIndex, depth
						{
							position87, tokenIndex87, depth87 := position, tokenIndex, depth
							if buffer[position] != rune('\n') {
								goto l87
							}
							position++
							goto l85
						l87:
							position, tokenIndex, depth = position87, tokenIndex87, depth87
						}
						if !matchDot() {
							goto l85
						}
						goto l84
					l85:
						position, tokenIndex, depth = position85, tokenIndex85, depth85
					}
					depth--
					add(rulePegText, position83)
				}
				if !_rules[ruleAction4]() {
					goto l81
				}
				depth--
				add(ruleColumnDescription, position82)
			}
			return true
		l81:
			position, tokenIndex, depth = position81, tokenIndex81, depth81
			return false
		},
		/* 12 dot <- <'.'> */
		func() bool {
			position88, tokenIndex88, depth88 := position, tokenIndex, depth
			{
				position89 := position
				depth++
				if buffer[position] != rune('.') {
					goto l88
				}
				position++
				depth--
				add(ruledot, position89)
			}
			return true
		l88:
			position, tokenIndex, depth = position88, tokenIndex88, depth88
			return false
		},
		/* 13 ColumnName <- <(<([a-z] / [A-Z] / '_')+> Action5)> */
		func() bool {
			position90, tokenIndex90, depth90 := position, tokenIndex, depth
			{
				position91 := position
				depth++
				{
					position92 := position
					depth++
					{
						position95, tokenIndex95, depth95 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l96
						}
						position++
						goto l95
					l96:
						position, tokenIndex, depth = position95, tokenIndex95, depth95
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l97
						}
						position++
						goto l95
					l97:
						position, tokenIndex, depth = position95, tokenIndex95, depth95
						if buffer[position] != rune('_') {
							goto l90
						}
						position++
					}
				l95:
				l93:
					{
						position94, tokenIndex94, depth94 := position, tokenIndex, depth
						{
							position98, tokenIndex98, depth98 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l99
							}
							position++
							goto l98
						l99:
							position, tokenIndex, depth = position98, tokenIndex98, depth98
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l100
							}
							position++
							goto l98
						l100:
							position, tokenIndex, depth = position98, tokenIndex98, depth98
							if buffer[position] != rune('_') {
								goto l94
							}
							position++
						}
					l98:
						goto l93
					l94:
						position, tokenIndex, depth = position94, tokenIndex94, depth94
					}
					depth--
					add(rulePegText, position92)
				}
				if !_rules[ruleAction5]() {
					goto l90
				}
				depth--
				add(ruleColumnName, position91)
			}
			return true
		l90:
			position, tokenIndex, depth = position90, tokenIndex90, depth90
			return false
		},
		/* 14 ColumnDef <- <(ColumnName (Space* ColumnType)?)> */
		func() bool {
			position101, tokenIndex101, depth101 := position, tokenIndex, depth
			{
				position102 := position
				depth++
				if !_rules[ruleColumnName]() {
					goto l101
				}
				{
					position103, tokenIndex103, depth103 := position, tokenIndex, depth
				l105:
					{
						position106, tokenIndex106, depth106 := position, tokenIndex, depth
						if !_rules[ruleSpace]() {
							goto l106
						}
						goto l105
					l106:
						position, tokenIndex, depth = position106, tokenIndex106, depth106
					}
					if !_rules[ruleColumnType]() {
						goto l103
					}
					goto l104
				l103:
					position, tokenIndex, depth = position103, tokenIndex103, depth103
				}
			l104:
				depth--
				add(ruleColumnDef, position102)
			}
			return true
		l101:
			position, tokenIndex, depth = position101, tokenIndex101, depth101
			return false
		},
		/* 15 RightArrow <- <(RightDotArrow / RightLineArrow)> */
		func() bool {
			position107, tokenIndex107, depth107 := position, tokenIndex, depth
			{
				position108 := position
				depth++
				{
					position109, tokenIndex109, depth109 := position, tokenIndex, depth
					if !_rules[ruleRightDotArrow]() {
						goto l110
					}
					goto l109
				l110:
					position, tokenIndex, depth = position109, tokenIndex109, depth109
					if !_rules[ruleRightLineArrow]() {
						goto l107
					}
				}
			l109:
				depth--
				add(ruleRightArrow, position108)
			}
			return true
		l107:
			position, tokenIndex, depth = position107, tokenIndex107, depth107
			return false
		},
		/* 16 ColumnType <- <(<(!('-' / ':' / '.' / '\n') .)+> Action6)> */
		func() bool {
			position111, tokenIndex111, depth111 := position, tokenIndex, depth
			{
				position112 := position
				depth++
				{
					position113 := position
					depth++
					{
						position116, tokenIndex116, depth116 := position, tokenIndex, depth
						{
							position117, tokenIndex117, depth117 := position, tokenIndex, depth
							if buffer[position] != rune('-') {
								goto l118
							}
							position++
							goto l117
						l118:
							position, tokenIndex, depth = position117, tokenIndex117, depth117
							if buffer[position] != rune(':') {
								goto l119
							}
							position++
							goto l117
						l119:
							position, tokenIndex, depth = position117, tokenIndex117, depth117
							if buffer[position] != rune('.') {
								goto l120
							}
							position++
							goto l117
						l120:
							position, tokenIndex, depth = position117, tokenIndex117, depth117
							if buffer[position] != rune('\n') {
								goto l116
							}
							position++
						}
					l117:
						goto l111
					l116:
						position, tokenIndex, depth = position116, tokenIndex116, depth116
					}
					if !matchDot() {
						goto l111
					}
				l114:
					{
						position115, tokenIndex115, depth115 := position, tokenIndex, depth
						{
							position121, tokenIndex121, depth121 := position, tokenIndex, depth
							{
								position122, tokenIndex122, depth122 := position, tokenIndex, depth
								if buffer[position] != rune('-') {
									goto l123
								}
								position++
								goto l122
							l123:
								position, tokenIndex, depth = position122, tokenIndex122, depth122
								if buffer[position] != rune(':') {
									goto l124
								}
								position++
								goto l122
							l124:
								position, tokenIndex, depth = position122, tokenIndex122, depth122
								if buffer[position] != rune('.') {
									goto l125
								}
								position++
								goto l122
							l125:
								position, tokenIndex, depth = position122, tokenIndex122, depth122
								if buffer[position] != rune('\n') {
									goto l121
								}
								position++
							}
						l122:
							goto l115
						l121:
							position, tokenIndex, depth = position121, tokenIndex121, depth121
						}
						if !matchDot() {
							goto l115
						}
						goto l114
					l115:
						position, tokenIndex, depth = position115, tokenIndex115, depth115
					}
					depth--
					add(rulePegText, position113)
				}
				if !_rules[ruleAction6]() {
					goto l111
				}
				depth--
				add(ruleColumnType, position112)
			}
			return true
		l111:
			position, tokenIndex, depth = position111, tokenIndex111, depth111
			return false
		},
		/* 17 RightDotArrow <- <('.' '.' '>' Action7)> */
		func() bool {
			position126, tokenIndex126, depth126 := position, tokenIndex, depth
			{
				position127 := position
				depth++
				if buffer[position] != rune('.') {
					goto l126
				}
				position++
				if buffer[position] != rune('.') {
					goto l126
				}
				position++
				if buffer[position] != rune('>') {
					goto l126
				}
				position++
				if !_rules[ruleAction7]() {
					goto l126
				}
				depth--
				add(ruleRightDotArrow, position127)
			}
			return true
		l126:
			position, tokenIndex, depth = position126, tokenIndex126, depth126
			return false
		},
		/* 18 RightLineArrow <- <('-' '>' Action8)> */
		func() bool {
			position128, tokenIndex128, depth128 := position, tokenIndex, depth
			{
				position129 := position
				depth++
				if buffer[position] != rune('-') {
					goto l128
				}
				position++
				if buffer[position] != rune('>') {
					goto l128
				}
				position++
				if !_rules[ruleAction8]() {
					goto l128
				}
				depth--
				add(ruleRightLineArrow, position129)
			}
			return true
		l128:
			position, tokenIndex, depth = position128, tokenIndex128, depth128
			return false
		},
		/* 19 TargetTableName <- <(<([a-z] / [A-Z] / '_')+> Action9)> */
		func() bool {
			position130, tokenIndex130, depth130 := position, tokenIndex, depth
			{
				position131 := position
				depth++
				{
					position132 := position
					depth++
					{
						position135, tokenIndex135, depth135 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l136
						}
						position++
						goto l135
					l136:
						position, tokenIndex, depth = position135, tokenIndex135, depth135
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l137
						}
						position++
						goto l135
					l137:
						position, tokenIndex, depth = position135, tokenIndex135, depth135
						if buffer[position] != rune('_') {
							goto l130
						}
						position++
					}
				l135:
				l133:
					{
						position134, tokenIndex134, depth134 := position, tokenIndex, depth
						{
							position138, tokenIndex138, depth138 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l139
							}
							position++
							goto l138
						l139:
							position, tokenIndex, depth = position138, tokenIndex138, depth138
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l140
							}
							position++
							goto l138
						l140:
							position, tokenIndex, depth = position138, tokenIndex138, depth138
							if buffer[position] != rune('_') {
								goto l134
							}
							position++
						}
					l138:
						goto l133
					l134:
						position, tokenIndex, depth = position134, tokenIndex134, depth134
					}
					depth--
					add(rulePegText, position132)
				}
				if !_rules[ruleAction9]() {
					goto l130
				}
				depth--
				add(ruleTargetTableName, position131)
			}
			return true
		l130:
			position, tokenIndex, depth = position130, tokenIndex130, depth130
			return false
		},
		/* 20 TargetColumnName <- <(<([a-z] / [A-Z] / '_')+> Action10)> */
		func() bool {
			position141, tokenIndex141, depth141 := position, tokenIndex, depth
			{
				position142 := position
				depth++
				{
					position143 := position
					depth++
					{
						position146, tokenIndex146, depth146 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l147
						}
						position++
						goto l146
					l147:
						position, tokenIndex, depth = position146, tokenIndex146, depth146
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l148
						}
						position++
						goto l146
					l148:
						position, tokenIndex, depth = position146, tokenIndex146, depth146
						if buffer[position] != rune('_') {
							goto l141
						}
						position++
					}
				l146:
				l144:
					{
						position145, tokenIndex145, depth145 := position, tokenIndex, depth
						{
							position149, tokenIndex149, depth149 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l150
							}
							position++
							goto l149
						l150:
							position, tokenIndex, depth = position149, tokenIndex149, depth149
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l151
							}
							position++
							goto l149
						l151:
							position, tokenIndex, depth = position149, tokenIndex149, depth149
							if buffer[position] != rune('_') {
								goto l145
							}
							position++
						}
					l149:
						goto l144
					l145:
						position, tokenIndex, depth = position145, tokenIndex145, depth145
					}
					depth--
					add(rulePegText, position143)
				}
				if !_rules[ruleAction10]() {
					goto l141
				}
				depth--
				add(ruleTargetColumnName, position142)
			}
			return true
		l141:
			position, tokenIndex, depth = position141, tokenIndex141, depth141
			return false
		},
		/* 21 EOT <- <!.> */
		func() bool {
			position152, tokenIndex152, depth152 := position, tokenIndex, depth
			{
				position153 := position
				depth++
				{
					position154, tokenIndex154, depth154 := position, tokenIndex, depth
					if !matchDot() {
						goto l154
					}
					goto l152
				l154:
					position, tokenIndex, depth = position154, tokenIndex154, depth154
				}
				depth--
				add(ruleEOT, position153)
			}
			return true
		l152:
			position, tokenIndex, depth = position152, tokenIndex152, depth152
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
