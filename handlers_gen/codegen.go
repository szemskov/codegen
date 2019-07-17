package main

// код писать тут
import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"reflect"
	"strings"
	"text/template"
)

const (
	API_METHOD_PREFIX    = "// apigen:api"
	API_VALIDATOR_PREFIX = "apivalidator"
)

type ApiConfig struct {
	Url    string
	Auth   bool
	Method string
}

type requestParam struct {
	ParamName  string
	ParamType  string
	Validators string
	Parsers    string
}

type apiTplParams struct {
	ApiName string
	Cases   []handlerTplParams
}

type handlerTplParams struct {
	ApiName    string
	MethodName string
	ParamsName string
	Params     []requestParam
	Config     ApiConfig
}

var (
	apiTpl = template.Must(template.New("apiTpl").Parse(`
func (h *{{.ApiName}}) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    switch r.URL.Path {
{{range .Cases}}
    case "{{.Config.Url}}":
		{{if .Config.Auth}}
		token := r.Header.Get("X-Auth")
		if token != "100500" {
			writeJsonResponse(w, Response{"error": "unauthorized"}, http.StatusForbidden)
			return
		}
		{{end}}
		{{if ne .Config.Method ""}}
		if r.Method != "{{.Config.Method}}" {
			writeJsonResponse(w, Response{"error": "bad method"}, http.StatusNotAcceptable)
			return
		}
		{{end}}
        h.handler{{.MethodName}}(w, r)
{{end}}
    default:
        // 404
		writeJsonResponse(w, Response{"error": "unknown method"}, http.StatusNotFound)
    }
}
`))

	handlerTpl = template.Must(template.New("handlerTpl").Parse(`
func (h *{{.ApiName}}) handler{{.MethodName}}(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := http.StatusOK
    errorMessage := ""
	
    // валидирование параметров
	// заполнение структуры params
	params := {{.ParamsName}}{}
    paramName := ""

	{{range .Params}}

    paramName = strings.ToLower("{{.ParamName}}")
    {{.Parsers}}

    {{ if eq .ParamType "string" }}
    params.{{.ParamName}} = r.FormValue(paramName)
    {{end}}
    {{ if eq .ParamType "int" }}
    value, err := strconv.Atoi(r.FormValue(paramName))
    if err != nil {
      writeJsonResponse(w, Response{"error": paramName + " must be int"}, http.StatusBadRequest)
      return
	}
	params.{{.ParamName}} = value
    {{end}}
	{{.Validators}}
    {{end}}

	result, err := h.{{.MethodName}}(ctx, params)

	// прочие обработки
	if err != nil {
		switch err.(type) {
		case ApiError:
			err := err.(ApiError)
			status = err.HTTPStatus
			errorMessage = err.Error()
		default:
            status = http.StatusInternalServerError
			errorMessage = err.Error()
		} 

		writeJsonResponse(w, Response{
			"error": errorMessage,
    	}, status)
		return 
	}

	writeJsonResponse(w, Response{
        "error": "",
		"response": result,
    }, status)
}
`))
	requiredValidatorTpl = template.Must(template.New("requiredValidatorTpl").Parse(`
    if ok, err := ({{.Validator}}{"{{.Constraint}}", paramName}).Validate(params.{{.FieldName}}); !ok && err != nil {
      writeJsonResponse(w, Response{
        "error": err.Error(),
      }, http.StatusBadRequest)
      return
    }
`))
	limitValidatorTpl = template.Must(template.New("limitValidatorTpl").Parse(`
    if ok, err := ({{.Validator}}{"{{.Constraint}}", paramName, {{.Limit}} }).Validate(params.{{.FieldName}}); !ok && err != nil {
      writeJsonResponse(w, Response{
        "error": err.Error(),
      }, http.StatusBadRequest)
      return
    }
`))
	enumValidatorTpl = template.Must(template.New("enumValidatorTpl").Parse(`
    if ok, err := ({{.Validator}}{"{{.Constraint}}", paramName, map[string]int{ {{.List}} }}).Validate(params.{{.FieldName}}); !ok && err != nil {
      writeJsonResponse(w, Response{
        "error": err.Error(),
      }, http.StatusBadRequest)
      return
    }
`))
)

func getParsersByTag(fieldName, fieldType, tag string) string {
	parsers := ""
	constraints := strings.Split(tag, ",")

	for _, constraint := range constraints {
		out := new(bytes.Buffer)

		if strings.Index(constraint, "paramname") == 0 {
			value := strings.Split(constraint, "=")[1]
			out.Write([]byte(fmt.Sprintf("paramName = \"%s\"", value)))
		}

		parsers += out.String()
	}

	return parsers
}

func getValidatorsByTag(fieldName, fieldType, tag string) string {
	validators := ""
	constraints := strings.Split(tag, ",")

	for _, constraint := range constraints {
		out := new(bytes.Buffer)

		if strings.Index(constraint, "required") == 0 {
			requiredValidatorTpl.Execute(out, map[string]interface{}{
				"Validator":  "RequiredValidator",
				"FieldName":  fieldName,
				"FieldType":  fieldType,
				"Constraint": constraint,
			})
		}

		if strings.Index(constraint, "min") == 0 {
			limitValidatorTpl.Execute(out, map[string]interface{}{
				"Validator":  strings.Title(fieldType) + "MinValidator",
				"FieldName":  fieldName,
				"FieldType":  fieldType,
				"Constraint": constraint,
				"Limit":      strings.Split(constraint, "=")[1],
			})
		}

		if strings.Index(constraint, "max") == 0 {
			limitValidatorTpl.Execute(out, map[string]interface{}{
				"Validator":  strings.Title(fieldType) + "MaxValidator",
				"FieldName":  fieldName,
				"FieldType":  fieldType,
				"Constraint": constraint,
				"Limit":      strings.Split(constraint, "=")[1],
			})
		}

		if strings.Index(constraint, "enum") == 0 {
			if strings.Contains(tag, "default") {
				value := strings.Split(tag, "=")[2]
				out.Write([]byte(fmt.Sprintf(`
	if params.%s == "" {
	 params.%s = "%s"
	}`, fieldName, fieldName, value)))
			}

			enumValidatorTpl.Execute(out, map[string]interface{}{
				"Validator":  "EnumValidator",
				"FieldName":  fieldName,
				"FieldType":  fieldType,
				"Constraint": constraint,
				"List":       "\"" + strings.Join(strings.Split(strings.Split(constraint, "=")[1], "|"), "\":1, \"") + "\": 1",
			})
		}

		validators += out.String()
	}

	return validators
}

func main() {
	paramsMap := make(map[string][]requestParam, 10)
	handlerMap := make(map[string][]handlerTplParams, 10)
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	out, _ := os.Create(os.Args[2])

	fmt.Fprintln(out, `package `+node.Name.Name)
	fmt.Fprintln(out) // empty line
	fmt.Fprintln(out, `import "net/http"`)
	fmt.Fprintln(out, `import "encoding/json"`)
	fmt.Fprintln(out, `import "strconv"`)
	fmt.Fprintln(out, `import "strings"`)
	fmt.Fprintln(out, `import "fmt"`)
	fmt.Fprintln(out) // empty line

	fmt.Fprintln(out, `
type Response map[string]interface{}

func writeJsonResponse(w http.ResponseWriter, response interface{}, status int) {
  jsonResponse, _ := json.Marshal(response)
  w.WriteHeader(status)
  _, err := w.Write(jsonResponse)
  if err != nil {
    panic("Unexpected error")
  }
}

type IntMaxValidator struct {
  rule string
  paramName string
  Max int
}

type IntMinValidator struct {
  rule string
  paramName string
  Min int
}

type StringMaxValidator struct {
  rule string
  paramName string
  Max int
}

type StringMinValidator struct {
  rule string
  paramName string
  Min int
}

type EnumValidator struct {
  rule string
  paramName string
  list map[string]int
}

type RequiredValidator struct {
  rule string
  paramName string
}

func (v IntMaxValidator) Validate(val interface{}) (bool, error) {
  num := val.(int)

  if num > v.Max {
    return false, fmt.Errorf(v.paramName + " must be <= %v", v.Max)
  }
  
  return true, nil
}

func (v IntMinValidator) Validate(val interface{}) (bool, error) {
  num := val.(int)

  if num < v.Min {
    return false, fmt.Errorf(v.paramName + " must be >= %v", v.Min)
  }
  
  return true, nil
}

func (v StringMaxValidator) Validate(val interface{}) (bool, error) {
  l := len(val.(string))

  if v.Max > 0 && l > v.Max {
    return false, fmt.Errorf(v.paramName + " must be <= %v", v.Max)
  }
  
  return true, nil
}

func (v StringMinValidator) Validate(val interface{}) (bool, error) {
  l := len(val.(string))

  if v.Min > 0 && l < v.Min {
    return false, fmt.Errorf(v.paramName + " len must be >= %v", v.Min)
  }
  
  return true, nil
}

func (v EnumValidator) Validate(val interface{}) (bool, error) {
  item := val.(string)

  if _,ok := v.list[item]; !ok {
	return false, fmt.Errorf(v.paramName + " must be one of [" + strings.Join(strings.Split(strings.Split(v.rule, "=")[1], "|"), ", ") + "]")
  }
  
  return true, nil
}

func (v RequiredValidator) Validate(val interface{}) (bool, error) {
  l := len(val.(string))

  if l == 0 {
	return false, fmt.Errorf(v.paramName + " must me not empty")
  }
  
  return true, nil
}`)

	// разбираем все структуры с параметрами API handler-ов
	for _, f := range node.Decls {
		g, ok := f.(*ast.GenDecl)
		if !ok {
			fmt.Printf("SKIP %T is not *ast.GenDecl\n", f)
			continue
		}

		for _, spec := range g.Specs {
			currType, ok := spec.(*ast.TypeSpec)
			if !ok {
				fmt.Printf("SKIP %T is not ast.TypeSpec\n", spec)
				continue
			}

			currStruct, ok := currType.Type.(*ast.StructType)
			if !ok {
				fmt.Printf("SKIP %T is not ast.StructType\n", currStruct)
				continue
			}

		FIELDS_LOOP:
			for _, field := range currStruct.Fields.List {
				if field.Tag == nil {
					continue FIELDS_LOOP
				}

				tag := reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1])
				if tag.Get(API_VALIDATOR_PREFIX) == "" {
					continue FIELDS_LOOP
				}

				fieldName := field.Names[0].Name
				fieldType := field.Type.(*ast.Ident).Name

				fmt.Printf("\tparse tag for field %s.%s %+v\n", currType.Name.Name, fieldName, tag)

				switch fieldType {
				case "int", "string":
					paramsMap[currType.Name.Name] = append(paramsMap[currType.Name.Name], requestParam{fieldName, fieldType, getValidatorsByTag(fieldName, fieldType, tag.Get(API_VALIDATOR_PREFIX)), getParsersByTag(fieldName, fieldType, tag.Get(API_VALIDATOR_PREFIX))})
				default:
					continue FIELDS_LOOP
				}
			}
		}
	}
	// делаем обертки для API с обертками в виде http обработчиков
	for _, f := range node.Decls {
		g, ok := f.(*ast.FuncDecl)

		// нам нужны только методы для API
		if !ok {
			fmt.Printf("SKIP %T is not *ast.FuncDecl\n", f)
			continue
		}

		// пропускаем все методы без меток нашего API
		if g.Doc == nil {
			fmt.Printf("SKIP func %#v doesnt have comments\n", g.Name.Name)
			continue
		}

		// если перед функцией есть коммент, то проверяем наличие нашей метки в этом комментарии
		needCodegen := false
		apiConfig := &ApiConfig{}
		for _, comment := range g.Doc.List {
			needCodegen = needCodegen || strings.HasPrefix(comment.Text, API_METHOD_PREFIX)
			strConfig := strings.TrimSpace(strings.Replace(comment.Text, API_METHOD_PREFIX, "", 1))

			err := json.Unmarshal([]byte(strConfig), apiConfig)
			if err != nil {
				fmt.Printf("SKIP func %#v couldnt getting api conf\n", g.Name.Name)
			}
		}
		if !needCodegen {
			fmt.Printf("SKIP func %#v doesnt have api mark\n", g.Name.Name)
			continue
		}

		apiName := g.Recv.List[0].Type.(*ast.StarExpr).X.(*ast.Ident).Name
		paramsName := g.Type.Params.List[1].Type.(*ast.Ident).Name
		handlerMap[apiName] = append(
			handlerMap[apiName],
			handlerTplParams{
				apiName,
				g.Name.Name,
				paramsName,
				paramsMap[paramsName],
				*apiConfig,
			})
		// парсим конфигурацию метода из комментария
		fmt.Printf("type: %T api: %s method: %s config:%#v\n", g, apiName, g.Name.Name, apiConfig)
	}

	// записыва
	for apiName, methodList := range handlerMap {
		apiTpl.Execute(out, apiTplParams{apiName, methodList})

		for _, tplParams := range methodList {
			handlerTpl.Execute(out, tplParams)
		}
	}
}
