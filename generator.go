package mswagger

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// It must return true if funcDeclaration is controller. We will try to parse only comments before controllers
func IsController(funcDeclaration *ast.FuncDecl, controllerClass string) bool {
	if len(controllerClass) == 0 {
		// Search every method
		return true
	}
	if funcDeclaration.Recv != nil && len(funcDeclaration.Recv.List) > 0 {
		if starExpression, ok := funcDeclaration.Recv.List[0].Type.(*ast.StarExpr); ok {
			receiverName := fmt.Sprint(starExpression.X)
			matched, err := regexp.MatchString(string(controllerClass), receiverName)
			if err != nil {
				log.Fatalf("The -controllerClass argument is not a valid regular expression: %v\n", err)
			}
			return matched
		}
	}
	return false
}

func InitParser(controllerClass, ignore string) *Parser {
	parser := NewParser()

	parser.ControllerClass = controllerClass
	parser.IsController = IsController
	parser.Ignore = ignore

	parser.TypesImplementingMarshalInterface["NullString"] = "string"
	parser.TypesImplementingMarshalInterface["NullInt64"] = "int"
	parser.TypesImplementingMarshalInterface["NullFloat64"] = "float"
	parser.TypesImplementingMarshalInterface["NullBool"] = "bool"

	return parser
}

type Params struct {
	ApiPackage, MainApiFile, OutputFormat, OutputPath, ControllerClass, Ignore string
}

func Run(params Params) error {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		return errors.New("$GOPATH environment variable is empty.")
	}

	var err error

	dirname, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return err
	}
	apiPackage := dirname[len(gopath)+5:]

	defaultParams := Params{
		ApiPackage:      apiPackage,
		MainApiFile:     apiPackage + "/main.go",
		OutputFormat:    "swagger",             // Current only swagger
		OutputPath:      "swagger-ui/index.js", // folder path
		ControllerClass: "",
		Ignore:          "swagger",
	}

	if params.ApiPackage == "" {
		params.ApiPackage = defaultParams.ApiPackage
	}
	if params.MainApiFile == "" {
		params.MainApiFile = defaultParams.MainApiFile
	}
	// if params.OutputFormat == "" {
	// 	params.OutputFormat = defaultParams.OutputFormat
	// }
	params.OutputFormat = defaultParams.OutputFormat
	if params.OutputPath == "" {
		params.OutputPath = defaultParams.OutputPath
	}
	if params.ControllerClass == "" {
		params.ControllerClass = defaultParams.ControllerClass
	}
	if params.Ignore == "" {
		params.Ignore = defaultParams.Ignore
	}

	parser := InitParser(params.ControllerClass, params.Ignore)
	// Support gopaths with multiple directories
	dirs := strings.Split(gopath, ":")
	if runtime.GOOS == "windows" {
		dirs = strings.Split(gopath, ";")
	}
	found := false
	for _, d := range dirs {
		apifile := path.Join(d, "src", params.MainApiFile)
		if _, err := os.Stat(apifile); err == nil {
			parser.ParseGeneralSwaggerInfo(apifile)
			found = true
		}
	}
	if found == false {
		if _, err := os.Stat(params.MainApiFile); err == nil {
			parser.ParseGeneralSwaggerInfo(params.MainApiFile)
		} else {
			apifile := path.Join(gopath, "src", params.MainApiFile)
			return fmt.Errorf("Could not find apifile %s to parse\n", apifile)
		}
	}

	parser.ParseApi(params.ApiPackage)

	// output, err := json.MarshalIndent(parser.Swagger, "", "  ")
	// fmt.Println(string(output))

	err = generateSwaggerUiFiles(parser, params.OutputPath)

	return err
}

func generateSwaggerUiFiles(parser *Parser, OutputPath string) error {
	fd, err := os.Create(OutputPath)
	if err != nil {
		return fmt.Errorf("Can not create the master index.json file: %v\n", err)
	}
	defer fd.Close()

	output, err := json.MarshalIndent(parser.Swagger, "", "  ")
	if err != nil {
		return err
	}
	fd.WriteString(string(output))

	// for apiKey, apiDescription := range parser.TopLevelApis {
	// 	err = os.MkdirAll(path.Join(outputSpec, apiKey), 0777)
	// 	if err != nil {
	// 		return err
	// 	}
	//
	// 	fd, err = os.Create(path.Join(outputSpec, apiKey, "index.json"))
	// 	if err != nil {
	// 		return fmt.Errorf("Can not create the %s/index.json file: %v\n", apiKey, err)
	// 	}
	// 	defer fd.Close()
	//
	// 	json, err := json.MarshalIndent(apiDescription, "", "    ")
	// 	if err != nil {
	// 		return fmt.Errorf("Can not serialise []ApiDescription to JSON: %v\n", err)
	// 	}
	//
	// 	fd.Write(json)
	// 	// log.Printf("Wrote %v/index.json", apiKey)
	// }

	return nil
}
