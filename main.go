package main

import (
	"os"
	"text/template"
	"log"
	"io/ioutil"
)

type LineType int

const (
	_ LineType = iota
	NormalLine
	DotLine
)

type Relation struct {
	LineType LineType
	TableName  string
	ColumnName string
}

func (r Relation) LineStyleLiteral() string {
	switch r.LineType {
	case NormalLine:
		return "solid"
	case DotLine:
		return "dotted"
	}
	return "solid"
}

type Column struct {
	Name     string
	Relation *Relation
	Description string
	Type string
}

type Table struct {
	Name    string
	Description string
	Columns []Column
}

func (t Table) ColumnsWithRelation() []Column {
	ret := make([]Column, 0)
	for _, c := range t.Columns {
		if c.Relation != nil {
			ret = append(ret, c)
		}
	}
	return ret
}

func ReadStdin() string {
	buf, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err.Error())
	}
	return string(buf)
}

func main() {
	//const s string = `
	//this {
	//id
	//that -> that.id
	//}
	//
	//that {
	//id
	//name
	//}
	//`  // 解析対象文字列
	c := ReadStdin()

	parser := &Parser{Buffer: c}  // 解析対象文字の設定
	parser.Init()                 // parser初期化
	err := parser.Parse()         // 解析
	if err != nil {
		log.Fatal(err.Error())
	} else {
		parser.Execute()          // アクション処理
	}

	////{{.Name}}[label="<B>{{.Name}}</B>{{range .Columns}}|<{{.Name}}>{{.Name}}{{end}}"];

	tmpl, err := template.New("test").Parse(`
digraph er {
	graph [rankdir=LR];
	ranksep="1.2";
	overlap=false;
	splines=true;
	sep="+30,30";
	node [shape=plaintext];
{{range .Tables}}
{{.Name}}[label=<
<TABLE STYLE="RADIAL" BORDER="1" CELLBORDER="0" CELLSPACING="1" ROWS="*">
  <TR><TD><B>{{.Name}}</B>{{if .Description}}<br />{{.Description}}{{end}}</TD></TR>
  {{range .Columns}}
    <TR><TD PORT="{{.Name}}" ALIGN="LEFT"><B>{{.Name}}</B> {{if .Type }}<I>{{.Type}}</I>{{end}} {{.Description}}</TD></TR>
  {{end}}
</TABLE>
>];
{{end}}

{{range $table := .Tables}}
{{range $column := $table.ColumnsWithRelation}}
{{$table.Name}}:{{$column.Name}} -> {{$column.Relation.TableName}}:{{$column.Relation.ColumnName}} [style="{{$column.Relation.LineStyleLiteral}}"];
{{end}}
{{end}}
}
		`)
	if err != nil {
		log.Fatalf(err.Error())
	}

	err = tmpl.Execute(os.Stdout, parser)
	if err != nil {
		log.Fatalf(err.Error())
	}
}
