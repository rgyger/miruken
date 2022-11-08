package test

import (
	"bytes"
	"fmt"
	"github.com/miruken-go/miruken"
	"github.com/miruken-go/miruken/api/json"
	"github.com/stretchr/testify/suite"
	"io"
	"strings"
	"testing"
)

type (
	PlayerMapper struct{}

	PlayerData struct {
		Id   int
		Name string
	}
)

func (m *PlayerMapper) ToPlayerJson(
	_*struct{
	    miruken.Maps
	    miruken.Format `as:"application/json"`
      }, data PlayerData,
) string {
	return fmt.Sprintf("{\"id\":%v,\"name\":\"%v\"}", data.Id, data.Name)
}

type JsonStdTestSuite struct {
	suite.Suite
}

func (suite *JsonStdTestSuite) Setup(specs ... any) (miruken.Handler, error) {
	return miruken.Setup(
		json.Feature(json.UseStandard()),
		miruken.HandlerSpecs(specs...))
}

func (suite *JsonStdTestSuite) TestJson() {
	suite.Run("Maps", func () {
		suite.Run("Json", func() {
			handler, _ := suite.Setup()

			suite.Run("ToJson", func() {
				data := struct{
					Name string
					Age  int
				}{
					"John Smith",
					23,
				}
				j, _, err := miruken.Map[string](handler, data, "application/json")
				suite.Nil(err)
				suite.Equal("{\"Name\":\"John Smith\",\"Age\":23}", j)
			})

			suite.Run("ToJsonWithOptions", func() {
				data := struct{
					Name string
					Age  int
				}{
					"Sarah Conner",
					38,
				}
				j, _, err := miruken.Map[string](
					miruken.BuildUp(handler, miruken.Options(json.StdOptions{Indent: "  "})),
					data, "application/json")
				suite.Nil(err)
				suite.Equal("{\n  \"Name\": \"Sarah Conner\",\n  \"Age\": 38\n}", j)
			})

			suite.Run("ToJsonMap", func() {
				data := map[string]any{
					"Id":    2,
					"Name": "George Best",
				}
				j, _, err := miruken.Map[string](handler, data, "application/json")
				suite.Nil(err)
				suite.Equal("{\"Id\":2,\"Name\":\"George Best\"}", j)
			})

			suite.Run("ToJsonStream", func() {
				data := struct{
					Name string
					Age  int
				}{
					"James Webb",
					85,
				}
				var b bytes.Buffer
				stream := io.Writer(&b)
				_, err := miruken.MapInto(handler, data, &stream, "application/json")
				suite.Nil(err)
				suite.Equal("{\"Name\":\"James Webb\",\"Age\":85}\n", b.String())
			})

			suite.Run("ToJsonStreamWithOptions", func() {
				data := struct{
					Name string
					Age  int
				}{
					"James Webb",
					85,
				}
				var b bytes.Buffer
				stream := io.Writer(&b)
				_, err := miruken.MapInto(
					miruken.BuildUp(handler, miruken.Options(json.StdOptions{Indent: "  "})),
					data, &stream, "application/json")
				suite.Nil(err)
				suite.Equal("{\n  \"Name\": \"James Webb\",\n  \"Age\": 85\n}\n", b.String())
			})

			suite.Run("ToJsonOverride", func() {
				handler, _ := suite.Setup(&PlayerMapper{})

				data := PlayerData{
					Id:   1,
					Name: "Tim Howard",
				}

				j, _, err := miruken.Map[string](handler, data, "application/json")
				suite.Nil(err)
				suite.Equal("{\"id\":1,\"name\":\"Tim Howard\"}", j)
			})

			suite.Run("FromJson", func() {
				type Data struct {
					Name string
					Age  int
				}
				j := "{\"Name\":\"Ralph Hall\",\"Age\":84}"
				data, _, err := miruken.Map[Data](handler, j, "application/json")
				suite.Nil(err)
				suite.Equal("Ralph Hall", data.Name)
				suite.Equal(84, data.Age)
			})

			suite.Run("FromJsonMap", func() {
				j := "{\"Name\":\"Ralph Hall\",\"Age\":84}"
				data, _,  err := miruken.Map[map[string]any](handler, j, "application/json")
				suite.Nil(err)
				suite.Equal(84.0, data["Age"])
				suite.Equal("Ralph Hall", data["Name"])
			})

			suite.Run("FromJsonStream", func() {
				type Data struct {
					Name string
					Age  int
				}
				stream := strings.NewReader("{\"Name\":\"Ralph Hall\",\"Age\":84}")
				data, _, err := miruken.Map[Data](handler, stream, "application/json")
				suite.Nil(err)
				suite.Equal("Ralph Hall", data.Name)
				suite.Equal(84, data.Age)
			})
		})
	})
}

func TestJsonStdTestSuite(t *testing.T) {
	suite.Run(t, new(JsonStdTestSuite))
}
