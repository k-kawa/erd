package main

import (
	"os"
	"text/template"
	"log"
	"io/ioutil"

	"github.com/urfave/cli"
	"io"
	"encoding/json"
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

type ParsedData interface {
	Tables() []Table
}

func (p Parser) Tables() []Table {
	return p.tables
}

func ExportDot(p ParsedData, wr io.Writer) error {
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
		return err
	}

	err = tmpl.Execute(os.Stdout, p)
	if err != nil {
		return err
	}

	return nil
}

func ExportJson(p ParsedData, wr io.Writer) error {
	data, err := json.Marshal(p.Tables())
	if err != nil {
		return err
	}

	if _, err := wr.Write(data); err != nil {
		return err
	}
	return nil
}

func main() {

	app := cli.NewApp()
	app.Name = "erd"
	app.Usage = "Yet another ER Diagram Maker"
	app.Version = "0.0.1"
	app.Commands = []cli.Command{
		{
			Name: "convert",
			Aliases: []string{"c"},
			Usage: "convert erd file to dot/json",
			Flags: []cli.Flag {
				cli.StringFlag{
					Name: "outformat",
					Value: "dot",
					Usage: "output format. dot and json is available.",
				},
			},
			Action: func(c *cli.Context) error {
				text := ReadStdin()

				parser := &Parser{Buffer: text}  // 解析対象文字の設定
				parser.Init()                 // parser初期化
				err := parser.Parse()         // 解析
				if err != nil {
					return cli.NewExitError(err.Error(), 1)
				}

				parser.Execute()

				outFormat := c.String("outformat")
				if outFormat == "json" {
					err = ExportJson(parser, os.Stdout)
				} else {
					err = ExportDot(parser, os.Stdout)
				}
				if err != nil {
					return cli.NewExitError(err.Error(), 1)
				}
				return nil
			},
		},
	}

	app.Run(os.Args)
}
