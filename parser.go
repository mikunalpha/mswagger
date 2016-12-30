package mswagger

import (
	"errors"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

type Parser struct {
	// Listing                           *ResourceListing
	// TopLevelApis                      map[string]*ApiDeclaration
	Swagger                           *SwaggerObject
	PackagesCache                     map[string]map[string]*ast.Package
	CurrentPackage                    string
	TypeDefinitions                   map[string]map[string]*ast.TypeSpec
	PackagePathCache                  map[string]string
	PackageImports                    map[string]map[string][]string
	BasePath                          string
	ControllerClass                   string
	Ignore                            string
	IsController                      func(*ast.FuncDecl, string) bool
	TypesImplementingMarshalInterface map[string]string
}

func NewParser() *Parser {
	return &Parser{
		Swagger:                           &SwaggerObject{},
		PackagesCache:                     make(map[string]map[string]*ast.Package),
		TypeDefinitions:                   make(map[string]map[string]*ast.TypeSpec),
		PackagePathCache:                  make(map[string]string),
		PackageImports:                    make(map[string]map[string][]string),
		TypesImplementingMarshalInterface: make(map[string]string),
	}
}

// Read web/main.go to get Swagger info
func (parser *Parser) ParseGeneralSwaggerInfo(mainApiFile string) {

	fileSet := token.NewFileSet()
	fileTree, err := goparser.ParseFile(fileSet, mainApiFile, nil, goparser.ParseComments)
	if err != nil {
		log.Fatalf("Can not parse general API information: %v\n", err)
	}

	parser.BasePath = ""
	parser.Swagger.Swagger = SwaggerVersion
	parser.Swagger.Info = &InfoObject{
		Contact: &Contact{},
		License: &License{},
	}
	parser.Swagger.Paths = map[string]*PathItemObject{}
	if fileTree.Comments != nil {
		for _, comment := range fileTree.Comments {
			for _, commentLine := range strings.Split(comment.Text(), "\n") {
				attribute := strings.ToLower(strings.Split(commentLine, " ")[0])
				switch attribute {
				case "@version":
					parser.Swagger.Info.Version = strings.TrimSpace(commentLine[len(attribute):])
				case "@title":
					parser.Swagger.Info.Title = strings.TrimSpace(commentLine[len(attribute):])
				case "@description":
					parser.Swagger.Info.Description = strings.TrimSpace(commentLine[len(attribute):])
				case "@termsofserviceurl":
					parser.Swagger.Info.TermsOfService = strings.TrimSpace(commentLine[len(attribute):])
				case "@contactname":
					parser.Swagger.Info.Contact.Name = strings.TrimSpace(commentLine[len(attribute):])
				case "@contactemail":
					parser.Swagger.Info.Contact.Email = strings.TrimSpace(commentLine[len(attribute):])
				case "@contacturl":
					parser.Swagger.Info.Contact.URL = strings.TrimSpace(commentLine[len(attribute):])
				case "@licensename":
					parser.Swagger.Info.License.Name = strings.TrimSpace(commentLine[len(attribute):])
				case "@licenseurl":
					parser.Swagger.Info.License.URL = strings.TrimSpace(commentLine[len(attribute):])
				case "@basepath":
					parser.Swagger.BasePath = strings.TrimSpace(commentLine[len(attribute):])
				case "@schemes":
					parser.Swagger.Schemes = strings.Split(strings.Replace(strings.TrimSpace(commentLine[len(attribute):]), " ", "", -1), ",")
				}
			}
		}
	}
}

// Parase apis info
func (parser *Parser) ParseApi(packageNames string) {
	packages := parser.ScanPackages(strings.Split(packageNames, ","))
	for _, packageName := range packages {
		parser.ParseTypeDefinitions(packageName)
	}
	for _, packageName := range packages {
		parser.ParseApiDescription(packageName)
	}
}

func (parser *Parser) ParseTypeDefinitions(packageName string) {
	parser.CurrentPackage = packageName
	pkgRealPath := parser.GetRealPackagePath(packageName)
	if pkgRealPath == "" {
		return
	}
	//	log.Printf("Parse type definition of %#v\n", packageName)

	if _, ok := parser.TypeDefinitions[pkgRealPath]; !ok {
		parser.TypeDefinitions[pkgRealPath] = make(map[string]*ast.TypeSpec)
	}
	astPackages := parser.GetPackageAst(pkgRealPath)
	for _, astPackage := range astPackages {
		for _, astFile := range astPackage.Files {
			for _, astDeclaration := range astFile.Decls {
				if generalDeclaration, ok := astDeclaration.(*ast.GenDecl); ok && generalDeclaration.Tok == token.TYPE {
					for _, astSpec := range generalDeclaration.Specs {
						if typeSpec, ok := astSpec.(*ast.TypeSpec); ok {
							parser.TypeDefinitions[pkgRealPath][typeSpec.Name.String()] = typeSpec
						}
					}
				}
			}
		}
	}

	//log.Fatalf("Type definition parsed %#v\n", parser.ParseImportStatements(packageName))

	for importedPackage, _ := range parser.ParseImportStatements(packageName) {
		//log.Printf("Import: %v\n", importedPackage)
		parser.ParseTypeDefinitions(importedPackage)
	}
}

func (parser *Parser) ParseImportStatements(packageName string) map[string]bool {

	parser.CurrentPackage = packageName
	pkgRealPath := parser.GetRealPackagePath(packageName)

	imports := make(map[string]bool)
	astPackages := parser.GetPackageAst(pkgRealPath)

	parser.PackageImports[pkgRealPath] = make(map[string][]string)
	for _, astPackage := range astPackages {
		for _, astFile := range astPackage.Files {
			for _, astImport := range astFile.Imports {
				importedPackageName := strings.Trim(astImport.Path.Value, "\"")
				if !parser.isIgnoredPackage(importedPackageName) {
					realPath := parser.GetRealPackagePath(importedPackageName)
					//log.Printf("path: %#v, original path: %#v", realPath, astImport.Path.Value)
					if _, ok := parser.TypeDefinitions[realPath]; !ok {
						imports[importedPackageName] = true
						//log.Printf("Parse %s, Add new import definition:%s\n", packageName, astImport.Path.Value)
					}

					var importedPackageAlias string
					if astImport.Name != nil && astImport.Name.Name != "." && astImport.Name.Name != "_" {
						importedPackageAlias = astImport.Name.Name
					} else {
						importPath := strings.Split(importedPackageName, "/")
						importedPackageAlias = importPath[len(importPath)-1]
					}

					isExists := false
					for _, v := range parser.PackageImports[pkgRealPath][importedPackageAlias] {
						if v == importedPackageName {
							isExists = true
						}
					}

					if !isExists {
						parser.PackageImports[pkgRealPath][importedPackageAlias] = append(parser.PackageImports[pkgRealPath][importedPackageAlias], importedPackageName)
					}
				}
			}
		}
	}
	return imports
}

func (parser *Parser) ParseApiDescription(packageName string) {
	parser.CurrentPackage = packageName
	pkgRealPath := parser.GetRealPackagePath(packageName)

	astPackages := parser.GetPackageAst(pkgRealPath)
	for _, astPackage := range astPackages {
		for _, astFile := range astPackage.Files {
			for _, astDescription := range astFile.Decls {
				switch astDeclaration := astDescription.(type) {
				case *ast.FuncDecl:
					if parser.IsController(astDeclaration, parser.ControllerClass) {
						operation := NewOperationObject(parser, packageName)
						if astDeclaration.Doc != nil && astDeclaration.Doc.List != nil {
							for _, comment := range astDeclaration.Doc.List {
								if err := operation.ParseComment(comment.Text); err != nil {
									log.Printf("Can not parse comment for function: %v, package: %v, got error: %v\n", astDeclaration.Name.String(), packageName, err)
								}
							}
						}
						// if operation.Path != "" {
						// 	// parser.AddOperation(operation)
						// }
					}
				}
			}
			// for _, astComment := range astFile.Comments {
			// 	for _, commentLine := range strings.Split(astComment.Text(), "\n") {
			// 		parser.ParseSubApiDescription(commentLine)
			// 	}
			// }
		}
	}
}

func (parser *Parser) CheckRealPackagePath(packagePath string) string {
	packagePath = strings.Trim(packagePath, "\"")

	if cachedResult, ok := parser.PackagePathCache[packagePath]; ok {
		return cachedResult
	}

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		log.Fatalf("Please, set $GOPATH environment variable\n")
	}

	// first check GOPATH
	pkgRealpath := ""
	gopathsList := filepath.SplitList(gopath)
	for _, path := range gopathsList {
		if evalutedPath, err := filepath.EvalSymlinks(filepath.Join("vendor", packagePath)); err == nil {
			if _, err := os.Stat(evalutedPath); err == nil {
				pkgRealpath = evalutedPath
				break
			}
		} else if evalutedPath, err := filepath.EvalSymlinks(filepath.Join(path, "src", packagePath)); err == nil {
			if _, err := os.Stat(evalutedPath); err == nil {
				pkgRealpath = evalutedPath
				break
			} // Mikun for vendor
		}
	}

	// next, check GOROOT (/src)
	if pkgRealpath == "" {
		goroot := filepath.Clean(runtime.GOROOT())
		if goroot == "" {
			log.Fatalf("Please, set $GOROOT environment variable\n")
		}
		if evalutedPath, err := filepath.EvalSymlinks(filepath.Join(goroot, "src", packagePath)); err == nil {
			if _, err := os.Stat(evalutedPath); err == nil {
				pkgRealpath = evalutedPath
			}
		} else if evalutedPath, err := filepath.EvalSymlinks(filepath.Join(goroot, "src/vendor", packagePath)); err == nil {
			if _, err := os.Stat(evalutedPath); err == nil {
				pkgRealpath = evalutedPath
			}
		}

		// next, check GOROOT (/src/pkg) (for golang < v1.4)
		if pkgRealpath == "" {
			if evalutedPath, err := filepath.EvalSymlinks(filepath.Join(goroot, "src", "pkg", packagePath)); err == nil {
				if _, err := os.Stat(evalutedPath); err == nil {
					pkgRealpath = evalutedPath
				}
			}
		}
	}
	parser.PackagePathCache[packagePath] = pkgRealpath
	return pkgRealpath
}

func (parser *Parser) GetRealPackagePath(packagePath string) string {
	//fmt.Println("GetRealPackagePath", packagePath)
	pkgRealpath := parser.CheckRealPackagePath(packagePath)
	if pkgRealpath == "" {
		// log.Fatalf("Can not find package %s \n", packagePath)
		log.Printf("Can not find package %s \n", packagePath)
	}

	return pkgRealpath
}

func (parser *Parser) GetPackageAst(packagePath string) map[string]*ast.Package {
	//log.Printf("Parse %s package\n", packagePath)
	if cache, ok := parser.PackagesCache[packagePath]; ok {
		return cache
	} else {
		fileSet := token.NewFileSet()

		astPackages, err := goparser.ParseDir(fileSet, packagePath, ParserFileFilter, goparser.ParseComments)
		if err != nil {
			log.Fatalf("Parse of %s pkg cause error: %s\n", packagePath, err)
		}
		parser.PackagesCache[packagePath] = astPackages
		return astPackages
	}
}

func (parser *Parser) ScanPackages(packages []string) []string {
	res := make([]string, len(packages))
	existsPackages := make(map[string]bool)

	for _, packageName := range packages {
		if v, ok := existsPackages[packageName]; !ok || v == false {
			// Add package
			existsPackages[packageName] = true
			res = append(res, packageName)
			// get it's real path
			pkgRealPath := parser.GetRealPackagePath(packageName)
			// Then walk
			var walker filepath.WalkFunc = func(path string, info os.FileInfo, err error) error {
				if info.IsDir() {
					if idx := strings.Index(path, packageName); idx != -1 {
						pack := path[idx:]
						if v, ok := existsPackages[pack]; !ok || v == false {
							existsPackages[pack] = true
							res = append(res, pack)
						}
					}
				}
				return nil
			}
			filepath.Walk(pkgRealPath, walker)
		}
	}
	return res
}

func (parser *Parser) GetModelDefinition(model string, packageName string) *ast.TypeSpec {
	pkgRealPath := parser.CheckRealPackagePath(packageName)
	if pkgRealPath == "" {
		return nil
	}

	packageModels, ok := parser.TypeDefinitions[pkgRealPath]
	if !ok {
		return nil
	}
	astTypeSpec, _ := packageModels[model]
	return astTypeSpec
}

func (parser *Parser) FindModelDefinition(modelName string, currentPackage string) (*ast.TypeSpec, string) {
	var model *ast.TypeSpec
	var modelPackage string

	modelNameParts := strings.Split(modelName, ".")

	//if no dot in name - it can be only model from current package
	if len(modelNameParts) == 1 {
		modelPackage = currentPackage
		if model = parser.GetModelDefinition(modelName, currentPackage); model == nil {
			log.Fatalf("Can not find definition of %s model. Current package %s", modelName, currentPackage)
		}
	} else {
		//first try to assume what name is absolute
		absolutePackageName := strings.Join(modelNameParts[:len(modelNameParts)-1], "/")
		modelNameFromPath := modelNameParts[len(modelNameParts)-1]

		modelPackage = absolutePackageName
		if model = parser.GetModelDefinition(modelNameFromPath, absolutePackageName); model == nil {

			//can not get model by absolute name.
			if len(modelNameParts) > 2 {
				log.Fatalf("Can not find definition of %s model. Name looks like absolute, but model not found in %s package", modelNameFromPath, absolutePackageName)
			}

			// lets try to find it in imported packages
			pkgRealPath := parser.CheckRealPackagePath(currentPackage)
			if imports, ok := parser.PackageImports[pkgRealPath]; !ok {
				log.Fatalf("Can not find definition of %s model. Package %s dont import anything", modelNameFromPath, pkgRealPath)
			} else if relativePackage, ok := imports[modelNameParts[0]]; !ok {
				log.Fatalf("Package %s is not imported to %s, Imported: %#v\n", modelNameParts[0], currentPackage, imports)
			} else {
				var modelFound bool

				for _, packageName := range relativePackage {
					if model = parser.GetModelDefinition(modelNameFromPath, packageName); model != nil {
						modelPackage = packageName
						modelFound = true

						break
					}
				}

				if !modelFound {
					log.Fatalf("Can not find definition of %s model in package %s", modelNameFromPath, relativePackage)
				}
			}
		}
	}
	return model, modelPackage
}

func (parser *Parser) isIgnoredPackage(packageName string) bool {
	r, _ := regexp.Compile("appengine+")
	matched, err := regexp.MatchString(parser.Ignore, packageName)
	if err != nil {
		log.Fatalf("The -ignore argument is not a valid regular expression: %v\n", err)
	}
	return packageName == "C" || r.MatchString(packageName) || matched
}

func (parser *Parser) IsImplementMarshalInterface(typeName string) bool {
	_, ok := parser.TypesImplementingMarshalInterface[typeName]
	return ok
}

func ParserFileFilter(info os.FileInfo) bool {
	name := info.Name()
	return !info.IsDir() && !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func NewOperationObject(p *Parser, packageName string) *OperationObject {
	return &OperationObject{
		parser:      p,
		packageName: packageName,
		// Models:      make([]*Model, 0),
		Responses: map[string]*ResponseObject{},
	}
}

func (operation *OperationObject) ParseComment(comment string) error {
	commentLine := strings.TrimSpace(strings.TrimLeft(comment, "//"))
	if len(commentLine) == 0 {
		return nil
	}
	attribute := strings.Fields(commentLine)[0]
	switch strings.ToLower(attribute) {
	case "@router":
		if err := operation.ParseRouterComment(commentLine); err != nil {
			return err
		}
	case "@resource":
		resource := strings.TrimSpace(commentLine[len(attribute):])

		re := regexp.MustCompile(`([^\s]+)[\s]+"([^"]+)"`)
		if matches := re.FindStringSubmatch(resource); len(matches) == 3 {
			isExist := false
			for _, t := range operation.parser.Swagger.Tags {
				if t.Name == matches[1] {
					isExist = true
				}
			}
			if !isExist {
				operation.parser.Swagger.Tags = append(operation.parser.Swagger.Tags, &TagObject{
					Name:        matches[1],
					Description: matches[2],
				})
			}
			resource = matches[1]
		}

		// if string(resource[0]) == "/" {
		// 	resource = resource[1:]
		// }
		if resource == "" {
			resource = "others"
		}
		if !IsInStringList(operation.Tags, resource) {
			operation.Tags = append(operation.Tags, resource)
		}
	case "@title":
		operation.Summary = strings.TrimSpace(commentLine[len(attribute):])
	case "@description":
		operation.Description = strings.TrimSpace(commentLine[len(attribute):])
	case "@success", "@failure":
		if err := operation.ParseResponseComment(strings.TrimSpace(commentLine[len(attribute):])); err != nil {
			return err
		}
	case "@param":
		if err := operation.ParseParamComment(strings.TrimSpace(commentLine[len(attribute):])); err != nil {
			return err
		}
	case "@accept", "@consume":
		if err := operation.ParseAcceptComment(strings.TrimSpace(commentLine[len(attribute):])); err != nil {
			return err
		}
	case "@produce":
		if err := operation.ParseProduceComment(strings.TrimSpace(commentLine[len(attribute):])); err != nil {
			return err
		}
	}
	return nil
}

func (operation *OperationObject) ParseRouterComment(commentLine string) error {
	sourceString := strings.TrimSpace(commentLine[len("@Router"):])

	re := regexp.MustCompile(`([\w\.\/\-{}]+)[^\[]+\[([^\]]+)`)
	var matches []string

	if matches = re.FindStringSubmatch(sourceString); len(matches) != 3 {
		return fmt.Errorf("Can not parse router comment \"%s\", skipped.", commentLine)
	}

	if _, ok := operation.parser.Swagger.Paths[matches[1]]; !ok {
		operation.parser.Swagger.Paths[matches[1]] = &PathItemObject{}
	}

	switch strings.ToUpper(matches[2]) {
	case "GET":
		if operation.parser.Swagger.Paths[matches[1]].Get == nil {
			operation.parser.Swagger.Paths[matches[1]].Get = operation
		}
	case "POST":
		if operation.parser.Swagger.Paths[matches[1]].Post == nil {
			operation.parser.Swagger.Paths[matches[1]].Post = operation
		}
	case "PATCH":
		if operation.parser.Swagger.Paths[matches[1]].Patch == nil {
			operation.parser.Swagger.Paths[matches[1]].Patch = operation
		}
	case "PUT":
		if operation.parser.Swagger.Paths[matches[1]].Put == nil {
			operation.parser.Swagger.Paths[matches[1]].Put = operation
		}
	case "DELETE":
		if operation.parser.Swagger.Paths[matches[1]].Delete == nil {
			operation.parser.Swagger.Paths[matches[1]].Delete = operation
		}
	case "OPTIONS":
		if operation.parser.Swagger.Paths[matches[1]].Options == nil {
			operation.parser.Swagger.Paths[matches[1]].Options = operation
		}
	case "HEAD":
		if operation.parser.Swagger.Paths[matches[1]].Head == nil {
			operation.parser.Swagger.Paths[matches[1]].Head = operation
		}
	}

	return nil
}

func (operation *OperationObject) ParseResponseComment(commentLine string) error {
	re := regexp.MustCompile(`([\d]+)[\s]+([\w\{\}]+)[\s]+([\w\-\.\/]+)[^"]*(.*)?`)
	var matches []string

	if matches = re.FindStringSubmatch(commentLine); len(matches) != 5 {
		return fmt.Errorf("Can not parse response comment \"%s\", skipped.", commentLine)
	}

	var response *ResponseObject
	var code int
	if code, err := strconv.Atoi(matches[1]); err != nil {
		return errors.New("Success http code must be int")
	} else {
		operation.Responses[fmt.Sprint(code)] = &ResponseObject{
			Schema: &SchemaObject{},
		}
		response = operation.Responses[fmt.Sprint(code)]
	}
	response.Description = strings.Trim(matches[4], "\"")

	typeName, err := operation.registerType(matches[3])
	if err != nil {
		return err
	}
	//
	// response.Schema.Type = strings.Trim(matches[2], "{}")
	if _, ok := operation.parser.Swagger.Definitions[typeName]; ok {
		response.Schema.Ref = "#/definitions/" + typeName
	} else {
		response.Schema.Type = strings.Trim(matches[2], "{}")
		// response.Schema.Properties = operation.parser.Swagger.Definitions[typeName].Properties
	}

	if code == 200 {
		if matches[2] == "{array}" {
			// operation.SetItemsType(typeName)
			response.Schema.Type = "array"
			response.Schema.Items = &ReferenceObject{
				Ref: "#/definitions/" + typeName,
			}
		} else {
			response.Schema.Type = typeName
		}
	}

	// output, err := json.MarshalIndent(response, "", "  ")
	// fmt.Println(string(output))

	return nil
}

type Parameter struct {
	ParamType     string `json:"paramType"` // path,query,body,header,form
	Name          string `json:"name"`
	Description   string `json:"description"`
	DataType      string `json:"dataType"` // 1.2 needed?
	Type          string `json:"type"`     // integer
	Format        string `json:"format"`   // int64
	AllowMultiple bool   `json:"allowMultiple"`
	Required      bool   `json:"required"`
	Minimum       int    `json:"minimum"`
	Maximum       int    `json:"maximum"`
}

func (operation *OperationObject) ParseParamComment(commentLine string) error {
	swaggerParameter := ParameterObject{}
	paramString := commentLine

	re := regexp.MustCompile(`([-\w]+)[\s]+([\w]+)[\s]+([\w.]+)[\s]+([\w]+)[\s]+"([^"]+)"`)

	if matches := re.FindStringSubmatch(paramString); len(matches) != 6 {
		return fmt.Errorf("Can not parse param comment \"%s\", skipped.", paramString)
	} else {
		typeName, err := operation.registerType(matches[3])
		if err != nil {
			return err
		}

		swaggerParameter.Name = matches[1]
		swaggerParameter.In = matches[2]
		if IsBasicTypeSwaggerType(typeName) {
			swaggerParameter.Type = basicTypesSwaggerTypes[typeName]
			swaggerParameter.Format = basicTypesSwaggerFormats[typeName]
		} else {
			if _, ok := operation.parser.Swagger.Definitions[typeName]; ok {
				// swaggerParameter.Ref = "#/definitions/" + typeName
				swaggerParameter.Schema = &SchemaObject{
					Ref: "#/definitions/" + typeName,
				}
			} else {
				swaggerParameter.Type = typeName
			}
		}
		requiredText := strings.ToLower(matches[4])
		swaggerParameter.Required = (requiredText == "true" || requiredText == "required")
		swaggerParameter.Description = matches[5]

		operation.Parameters = append(operation.Parameters, swaggerParameter)
	}

	return nil
}

func (operation *OperationObject) ParseAcceptComment(commentLine string) error {
	accepts := strings.Split(commentLine, ",")
	for _, a := range accepts {
		switch a {
		case "json", "application/json":
			operation.Consumes = append(operation.Consumes, ContentTypeJson)
		case "xml", "text/xml":
			operation.Consumes = append(operation.Consumes, ContentTypeXml)
		case "plain", "text/plain":
			operation.Consumes = append(operation.Consumes, ContentTypePlain)
		case "html", "text/html":
			operation.Consumes = append(operation.Consumes, ContentTypeHtml)
		case "mpfd", "multipart/form-data":
			operation.Consumes = append(operation.Consumes, ContentTypeMultiPartFormData)
		}
	}
	return nil
}

func (operation *OperationObject) ParseProduceComment(commentLine string) error {
	produces := strings.Split(commentLine, ",")
	for _, a := range produces {
		switch a {
		case "json", "application/json":
			operation.Produces = append(operation.Produces, ContentTypeJson)
		case "xml", "text/xml":
			operation.Produces = append(operation.Produces, ContentTypeXml)
		case "plain", "text/plain":
			operation.Produces = append(operation.Produces, ContentTypePlain)
		case "html", "text/html":
			operation.Produces = append(operation.Produces, ContentTypeHtml)
		case "mpfd", "multipart/form-data":
			operation.Produces = append(operation.Produces, ContentTypeMultiPartFormData)
		}
	}
	return nil
}

var typeDefTranslations = map[string]string{}

var modelNamesPackageNames = map[string]string{}

// refer to builtin.go
var basicTypes = map[string]bool{
	"bool":       true,
	"uint":       true,
	"uint8":      true,
	"uint16":     true,
	"uint32":     true,
	"uint64":     true,
	"int":        true,
	"int8":       true,
	"int16":      true,
	"int32":      true,
	"int64":      true,
	"float32":    true,
	"float64":    true,
	"string":     true,
	"complex64":  true,
	"complex128": true,
	"byte":       true,
	"rune":       true,
	"uintptr":    true,
	"error":      true,
	"Time":       true,
	"file":       true,

	"undefined": true,
}

var basicTypesSwaggerTypes = map[string]string{
	"bool":    "boolean",
	"uint":    "integer",
	"uint8":   "integer",
	"uint16":  "integer",
	"uint32":  "integer",
	"uint64":  "integer",
	"int":     "integer",
	"int8":    "integer",
	"int16":   "integer",
	"int32":   "integer",
	"int64":   "integer",
	"float32": "number",
	"float64": "number",
	"string":  "string",
	"file":    "formData",
}

var basicTypesSwaggerFormats = map[string]string{
	"bool":    "boolean",
	"uint":    "integer",
	"uint8":   "int64",
	"uint16":  "int64",
	"uint32":  "int64",
	"uint64":  "int64",
	"int":     "int64",
	"int8":    "int64",
	"int16":   "int64",
	"int32":   "int64",
	"int64":   "int64",
	"float32": "float",
	"float64": "double",
	"string":  "string",
}

func IsBasicType(typeName string) bool {
	_, ok := basicTypes[typeName]
	return ok || strings.Contains(typeName, "interface")
}

func IsBasicTypeSwaggerType(typeName string) bool {
	_, ok := basicTypesSwaggerTypes[typeName]
	return ok || strings.Contains(typeName, "interface")
}

func (operation *OperationObject) registerType(typeName string) (string, error) {
	registerType := ""

	if translation, ok := typeDefTranslations[typeName]; ok {
		registerType = translation
	} else if IsBasicType(typeName) {
		registerType = typeName
	} else {
		model := NewModel(operation.parser)
		knownModelNames := map[string]bool{}

		err, innerModels := model.ParseModel(typeName, operation.parser.CurrentPackage, knownModelNames)
		// err, _ := model.ParseModel(typeName, operation.parser.CurrentPackage, knownModelNames)
		if err != nil {
			return registerType, err
		}
		if translation, ok := typeDefTranslations[typeName]; ok {
			registerType = translation
			// fmt.Println("## ", registerType)
		} else {
			registerType = model.Id

			// fmt.Println("@", model.Properties)
			if operation.parser.Swagger.Definitions == nil {
				operation.parser.Swagger.Definitions = map[string]*SchemaObject{}
			}

			if _, ok := operation.parser.Swagger.Definitions[registerType]; !ok {
				operation.parser.Swagger.Definitions[registerType] = &SchemaObject{
					Type:       "object",
					Required:   model.Required,
					Properties: map[string]interface{}{},
				}
			}

			for k, v := range model.Properties {
				operation.parser.Swagger.Definitions[registerType].Properties[k] = v
			}

			for _, m := range innerModels {
				registerType := m.Id // local var registerType
				if _, ok := operation.parser.Swagger.Definitions[registerType]; !ok {
					operation.parser.Swagger.Definitions[registerType] = &SchemaObject{
						Type:       "object",
						Required:   m.Required,
						Properties: map[string]interface{}{},
					}
				}

				for k, v := range m.Properties {
					operation.parser.Swagger.Definitions[registerType].Properties[k] = v
				}
				// fmt.Println("@@", m.Id)
			}
			// operation.Models = append(operation.Models, model)
			// operation.Models = append(operation.Models, innerModels...)
		}
	}

	return registerType, nil
}

type Model struct {
	Id         string                    `json:"id"`
	Required   []string                  `json:"required,omitempty"`
	Properties map[string]*ModelProperty `json:"properties"`
	parser     *Parser
}

type ModelProperty struct {
	Ref         string             `json:"$ref,omitempty"`
	Type        string             `json:"type"`
	Description string             `json:"description"`
	Format      string             `json:"format"`
	Items       ModelPropertyItems `json:"items,omitempty"`
}

func NewModelProperty() *ModelProperty {
	return &ModelProperty{}
}

type ModelPropertyItems struct {
	Ref  string `json:"$ref,omitempty"`
	Type string `json:"type,omitempty"`
}

func NewModel(p *Parser) *Model {
	return &Model{
		parser: p,
	}
}

// modelName is something like package.subpackage.SomeModel or just "subpackage.SomeModel"
func (m *Model) ParseModel(modelName string, currentPackage string, knownModelNames map[string]bool) (error, []*Model) {
	knownModelNames[modelName] = true
	//log.Printf("Before parse model |%s|, package: |%s|\n", modelName, currentPackage)

	astTypeSpec, modelPackage := m.parser.FindModelDefinition(modelName, currentPackage)

	modelNameParts := strings.Split(modelName, ".")
	m.Id = strings.Join(append(strings.Split(modelPackage, "/"), modelNameParts[len(modelNameParts)-1]), ".")

	if _, ok := modelNamesPackageNames[modelName]; !ok {
		modelNamesPackageNames[modelName] = m.Id
	}

	// fmt.Println("#", m.Id)

	var innerModelList []*Model
	if astTypeDef, ok := astTypeSpec.Type.(*ast.Ident); ok {
		typeDefTranslations[astTypeSpec.Name.String()] = astTypeDef.Name
	} else if astStructType, ok := astTypeSpec.Type.(*ast.StructType); ok {
		m.ParseFieldList(astStructType.Fields.List, modelPackage)
		usedTypes := make(map[string]bool)

		for _, property := range m.Properties {
			typeName := property.Type
			if typeName == "array" {
				if property.Items.Type != "" {
					typeName = property.Items.Type
				} else {
					typeName = property.Items.Ref
				}
			}
			if translation, ok := typeDefTranslations[typeName]; ok {
				typeName = translation
			}
			if IsBasicType(typeName) {
				if IsBasicTypeSwaggerType(typeName) {
					property.Format = basicTypesSwaggerFormats[typeName]
					if property.Type != "array" {
						property.Type = basicTypesSwaggerTypes[typeName]
					}
				}
				continue
			}
			if m.parser.IsImplementMarshalInterface(typeName) {
				continue
			}
			if _, exists := knownModelNames[typeName]; exists {
				// fmt.Println("@", typeName)
				if _, ok := modelNamesPackageNames[typeName]; ok {
					property.Ref = "#/definitions/" + modelNamesPackageNames[typeName]
					// property.Type = ""
				}
				continue
			}

			usedTypes[typeName] = true
		}

		//log.Printf("Before parse inner model list: %#v\n (%s)", usedTypes, modelName)
		innerModelList = make([]*Model, 0, len(usedTypes))

		for typeName, _ := range usedTypes {
			typeModel := NewModel(m.parser)
			if err, typeInnerModels := typeModel.ParseModel(typeName, modelPackage, knownModelNames); err != nil {
				//log.Printf("Parse Inner Model error %#v \n", err)
				return err, nil
			} else {
				for _, property := range m.Properties {
					if property.Type == "array" {
						if property.Items.Ref == typeName {
							property.Items.Ref = "#/definitions/" + typeModel.Id
						}
					} else {
						if property.Type == typeName {
							property.Ref = "#/definitions/" + typeModel.Id
							// property.Type = typeModel.Id
						} else {
							// fmt.Println(property.Type, "<>", typeName)
						}
					}
				}
				//log.Printf("Inner model %v parsed, parsing %s \n", typeName, modelName)
				if typeModel != nil {
					innerModelList = append(innerModelList, typeModel)
				}
				if typeInnerModels != nil && len(typeInnerModels) > 0 {
					innerModelList = append(innerModelList, typeInnerModels...)
				}
				//log.Printf("innerModelList: %#v\n, typeInnerModels: %#v, usedTypes: %#v \n", innerModelList, typeInnerModels, usedTypes)
			}
		}
		//log.Printf("After parse inner model list: %#v\n (%s)", usedTypes, modelName)
		// log.Fatalf("Inner model list: %#v\n", innerModelList)

	}

	//log.Printf("ParseModel finished %s \n", modelName)
	return nil, innerModelList
}

func (m *Model) ParseFieldList(fieldList []*ast.Field, modelPackage string) {
	if fieldList == nil {
		return
	}
	//log.Printf("ParseFieldList\n")

	m.Properties = make(map[string]*ModelProperty)
	for _, field := range fieldList {
		m.ParseModelProperty(field, modelPackage)
	}
}

func (m *Model) ParseModelProperty(field *ast.Field, modelPackage string) {
	var name string
	var innerModel *Model

	property := NewModelProperty()

	typeAsString := property.GetTypeAsString(field.Type)
	//log.Printf("Get type as string %s \n", typeAsString)

	// Sometimes reflection reports an object as "&{foo Bar}" rather than just "foo.Bar"
	// The next 2 lines of code normalize them to foo.Bar
	reInternalRepresentation := regexp.MustCompile("&\\{(\\w*) (\\w*)\\}")
	typeAsString = string(reInternalRepresentation.ReplaceAll([]byte(typeAsString), []byte("$1.$2")))

	if strings.HasPrefix(typeAsString, "[]") {
		property.Type = "array"
		property.SetItemType(typeAsString[2:])
		// if is Unsupported item type of list, ignore this property
		if property.Items.Type == "undefined" {
			property = nil
			return
		}
	} else if typeAsString == "time.Time" {
		property.Type = "Time"
	} else {
		property.Type = typeAsString
	}

	if len(field.Names) == 0 {
		if astSelectorExpr, ok := field.Type.(*ast.SelectorExpr); ok {
			packageName := modelPackage
			if astTypeIdent, ok := astSelectorExpr.X.(*ast.Ident); ok {
				packageName = astTypeIdent.Name
			}

			name = packageName + "." + strings.TrimPrefix(astSelectorExpr.Sel.Name, "*")
		} else if astTypeIdent, ok := field.Type.(*ast.Ident); ok {
			name = astTypeIdent.Name
		} else if astStarExpr, ok := field.Type.(*ast.StarExpr); ok {
			if astIdent, ok := astStarExpr.X.(*ast.Ident); ok {
				name = astIdent.Name
			}
		} else {
			log.Fatalf("Something goes wrong: %#v", field.Type)
		}
		innerModel = NewModel(m.parser)
		//log.Printf("Try to parse embeded type %s \n", name)
		//log.Fatalf("DEBUG: field: %#v\n, selector.X: %#v\n selector.Sel: %#v\n", field, astSelectorExpr.X, astSelectorExpr.Sel)
		knownModelNames := map[string]bool{}
		innerModel.ParseModel(name, modelPackage, knownModelNames)

		for innerFieldName, innerField := range innerModel.Properties {
			m.Properties[innerFieldName] = innerField
		}

		//log.Fatalf("Here %#v\n", field.Type)
		return
	} else {
		name = field.Names[0].Name
	}

	//log.Printf("ParseModelProperty: %s, CurrentPackage %s, type: %s \n", name, modelPackage, property.Type)
	//Analyse struct fields annotations
	if field.Tag != nil {
		structTag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
		var tagText string
		if thriftTag := structTag.Get("thrift"); thriftTag != "" {
			tagText = thriftTag
		}
		if tag := structTag.Get("json"); tag != "" {
			tagText = tag
		}

		tagValues := strings.Split(tagText, ",")
		var isRequired = false

		for _, v := range tagValues {
			if v != "" && v != "required" && v != "omitempty" {
				name = v
			}
			if v == "required" {
				isRequired = true
			}
			// We will not document at all any fields with a json tag of "-"
			if v == "-" {
				return
			}
		}
		if required := structTag.Get("required"); required != "" || isRequired {
			m.Required = append(m.Required, name)
		}
		if desc := structTag.Get("description"); desc != "" {
			property.Description = desc
		}
	}
	m.Properties[name] = property
}

func (p *ModelProperty) SetItemType(itemType string) {
	p.Items = ModelPropertyItems{}
	if IsBasicType(itemType) {
		if IsBasicTypeSwaggerType(itemType) {
			p.Items.Type = basicTypesSwaggerTypes[itemType]
		} else {
			p.Items.Type = "undefined"
			// p = &ModelProperty{}
		}
	} else {
		// p.Items.Ref = "#/definitions/" + itemType
		p.Items.Ref = itemType
	}
}

func (p *ModelProperty) GetTypeAsString(fieldType interface{}) string {
	var realType string
	if astArrayType, ok := fieldType.(*ast.ArrayType); ok {
		//		log.Printf("arrayType: %#v\n", astArrayType)
		realType = fmt.Sprintf("[]%v", p.GetTypeAsString(astArrayType.Elt))
	} else if astMapType, ok := fieldType.(*ast.MapType); ok {
		//		log.Printf("arrayType: %#v\n", astArrayType)
		realType = fmt.Sprintf("[]%v", p.GetTypeAsString(astMapType.Value))
	} else if _, ok := fieldType.(*ast.InterfaceType); ok {
		realType = "interface"
	} else {
		if astStarExpr, ok := fieldType.(*ast.StarExpr); ok {
			realType = fmt.Sprint(astStarExpr.X)
			//			log.Printf("Get type as string (star expression)! %#v, type: %s\n", astStarExpr.X, fmt.Sprint(astStarExpr.X))
		} else if astSelectorExpr, ok := fieldType.(*ast.SelectorExpr); ok {
			packageNameIdent, _ := astSelectorExpr.X.(*ast.Ident)
			realType = packageNameIdent.Name + "." + astSelectorExpr.Sel.Name

			//			log.Printf("Get type as string(selector expression)! X: %#v , Sel: %#v, type %s\n", astSelectorExpr.X, astSelectorExpr.Sel, realType)
		} else {
			//			log.Printf("Get type as string(no star expression)! %#v , type: %s\n", fieldType, fmt.Sprint(fieldType))
			realType = fmt.Sprint(fieldType)
		}
	}
	return realType
}
