package main

import (
	"testing"
	. "github.com/smartystreets/goconvey/convey"
)

func parse(t *testing.T, code string) (error, *Parser) {
	parser := &Parser{Buffer: code}
	parser.Init()                 // parser初期化
	err := parser.Parse()         // 解析
	parser.Execute()

	return err, parser
}

func TestSum(t *testing.T) {
	actual := 10 + 20
	expected := 30
	if actual != expected {
		t.Errorf("got %v\nwant %v", actual, expected)
	}
}

func TestSpec(t *testing.T) {
	Convey("Given some integer with a starting value", t, func() {
		x := 1

		Convey("The value should be greater by one", func() {
			So(x, ShouldEqual, 1)
		})
	})

	Convey("Simplest case", t, func() {
		err, parser := parse(t, `
devices {
  id
  user_id
  token
}`)
		So(err, ShouldBeNil)
		So(len(parser.Tables()), ShouldEqual, 1)
		So(len(parser.Tables()[0].Columns), ShouldEqual, 3)
	})

	Convey("Simplest Relation", t, func() {
		err, parser := parse(t, `
devices {
  id
  user_id -> users.id
  token
}

users {
  id
  name
}`)
		So(err, ShouldBeNil)
		So(len(parser.Tables()), ShouldEqual, 2)
		So(parser.Tables()[0].Columns[1].Relation.TableName, ShouldEqual, "users")
		So(parser.Tables()[0].Columns[1].Relation.ColumnName, ShouldEqual, "id")
		So(parser.Tables()[0].Columns[1].Relation.LineType, ShouldEqual, NormalLine)
	})

	Convey("Dotted Relation", t, func() {
		err, parser := parse(t, `
devices {
  id
  user_id ..> users.id
  token
}

users {
  id
  name
}`)
		So(err, ShouldBeNil)
		So(len(parser.Tables()), ShouldEqual, 2)
		So(parser.Tables()[0].Columns[1].Relation.TableName, ShouldEqual, "users")
		So(parser.Tables()[0].Columns[1].Relation.ColumnName, ShouldEqual, "id")
		So(parser.Tables()[0].Columns[1].Relation.LineType, ShouldEqual, DotLine)
	})

	Convey("Column Description with Relation", t, func() {
		err, parser := parse(t, `
devices {
  id
  user_id -> users.id : User of the device.
  token
}

users {
  id
  name
}`)
		So(err, ShouldBeNil)
		So(len(parser.Tables()), ShouldEqual, 2)
		So(parser.Tables()[0].Columns[1].Relation.TableName, ShouldEqual, "users")
		So(parser.Tables()[0].Columns[1].Relation.ColumnName, ShouldEqual, "id")
		So(parser.Tables()[0].Columns[1].Relation.LineType, ShouldEqual, NormalLine)
		So(parser.Tables()[0].Columns[1].Description, ShouldEqual, "User of the device.")
	})

	Convey("Column Description", t, func() {
		err, parser := parse(t, `
devices {
  id
  user_id
  token : User unique token.
}`)
		So(err, ShouldBeNil)
		So(len(parser.Tables()), ShouldEqual, 1)
		So(parser.Tables()[0].Columns[2].Description, ShouldEqual, "User unique token.")
	})

	Convey("Table Description", t, func() {
		err, parser := parse(t, `
devices : devices including iOS/Android {
  id
  user_id
  token
}`)
		So(err, ShouldBeNil)
		So(len(parser.Tables()), ShouldEqual, 1)
		So(parser.Tables()[0].Description, ShouldEqual, "devices including iOS/Android")
	})

	Convey("Column Type", t, func() {
		err, parser := parse(t, `
devices : devices including iOS/Android {
  id
  user_id BIGINT
  token
}`)
		So(err, ShouldBeNil)
		So(len(parser.Tables()), ShouldEqual, 1)
		So(parser.Tables()[0].Columns[1].Type, ShouldEqual, "BIGINT")
	})

	Convey("Column Type with relation", t, func() {
		err, parser := parse(t, `
devices : devices including iOS/Android {
  id
  user_id BIGINT -> users.id
  token
}`)
		So(err, ShouldBeNil)
		So(len(parser.Tables()), ShouldEqual, 1)
		So(parser.Tables()[0].Columns[1].Type, ShouldEqual, "BIGINT")
		So(parser.Tables()[0].Columns[1].Relation.TableName, ShouldEqual, "users")
		So(parser.Tables()[0].Columns[1].Relation.ColumnName, ShouldEqual, "id")
		So(parser.Tables()[0].Columns[1].Relation.LineType, ShouldEqual, NormalLine)
	})

	Convey("Column Type with relation and description", t, func() {
		err, parser := parse(t, `
devices : devices including iOS/Android {
  id
  user_id BIGINT -> users.id : The relation
  token
}`)
		So(err, ShouldBeNil)
		So(len(parser.Tables()), ShouldEqual, 1)
		So(parser.Tables()[0].Columns[1].Description, ShouldEqual, "The relation")
		So(parser.Tables()[0].Columns[1].Type, ShouldEqual, "BIGINT")
		So(parser.Tables()[0].Columns[1].Relation.TableName, ShouldEqual, "users")
		So(parser.Tables()[0].Columns[1].Relation.ColumnName, ShouldEqual, "id")
		So(parser.Tables()[0].Columns[1].Relation.LineType, ShouldEqual, NormalLine)
	})
}
