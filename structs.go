package mswagger

const SwaggerVersion = "2.0"

const (
	ContentTypeJson              = "application/json"
	ContentTypeXml               = "application/xml"
	ContentTypePlain             = "text/plain"
	ContentTypeHtml              = "text/html"
	ContentTypeMultiPartFormData = "multipart/form-data"
)

type SwaggerObject struct {
	Swagger             string                       `json:"swagger"`
	Info                *InfoObject                  `json:"info"`
	Host                string                       `json:"host,omitempty"`
	BasePath            string                       `json:"basePath,omitempty"`
	Schemes             []string                     `json:"schemes,omitempty"`
	Consumes            []string                     `json:"consumes,omitempty"`
	Produces            []string                     `json:"produces,omitempty"`
	Paths               map[string]*PathItemObject   `json:"paths"`
	Definitions         map[string]*SchemaObject     `json:"definitions,omitempty"`
	Parameters          map[string]interface{}       `json:"patameters,omitempty"`
	Responses           map[string]interface{}       `json:"responses,omitempty"`
	SecurityDefinitions map[string]interface{}       `json:"securityDefinitions,omitempty"`
	Security            map[string][]string          `json:"security,omitempty"`
	Tags                []*TagObject                 `json:"tags,omitempty"`
	ExternalDocs        *ExternalDocumentationObject `json:"externalDocs,omitempty"`
}

type InfoObject struct {
	Title          string   `json:"title"`
	Description    string   `json:"description,omitempty"`
	TermsOfService string   `json:"termsOfService,omitempty"`
	Contact        *Contact `json:"contact,omitempty"`
	License        *License `json:"license,omitempty"`
	Version        string   `json:"version,omitempty"`
}

type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

type License struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

type PathItemObject struct {
	Ref        string           `json:"$ref,omitempty"`
	Get        *OperationObject `json:"get,omitempty"`
	Put        *OperationObject `json:"put,omitempty"`
	Post       *OperationObject `json:"post,omitempty"`
	Delete     *OperationObject `json:"delete,omitempty"`
	Options    *OperationObject `json:"options,omitempty"`
	Head       *OperationObject `json:"head,omitempty"`
	Patch      *OperationObject `json:"patch,omitempty"`
	Parameters []interface{}    `json:"parameters,omitempty"`
}

type ParameterObject struct {
	Ref              string        `json:"$ref,omitempty"`
	Name             string        `json:"name"`
	In               string        `json:"in"`
	Description      string        `json:"description,omitempty"`
	Required         bool          `json:"required,omitempty"`
	Schema           *SchemaObject `json:"schema,omitempty"`
	Type             string        `json:"type,omitempty"`
	Format           string        `json:"format,omitempty"`
	AllowEmptyValue  bool          `json:"allowEmptyValue,omitempty"`
	Items            []interface{} `json:"items,omitempty"`
	CollectionFormat string        `json:"collectFormat,omitempty"`
	// TODO ...
}

type OperationObject struct {
	Tags         []string                     `json:"tags,omitempty"`
	Summary      string                       `json:"summary,omitempty"`
	Description  string                       `json:"description,omitempty"`
	ExternalDocs *ExternalDocumentationObject `json:"externalDocs,omitempty"`
	OperationId  string                       `json:"operationId,omitempty"`
	Consumes     []string                     `json:"consumes,omitempty"`
	Produces     []string                     `json:"produces,omitempty"`
	Parameters   []interface{}                `json:"parameters,omitempty"`
	Responses    map[string]*ResponseObject   `json:"responses,omitempty"`
	Schemes      []string                     `json:"schemes,omitempty"`
	Deprecated   bool                         `json:"deprecated,omitempty"`
	Security     map[string][]string          `json:"security,omitempty"`
	parser       *Parser
	packageName  string
}

type ReferenceObject struct {
	Ref string `json:"$ref,omitempty"`
}

// ResponseObject/ReferenceObject
type ResponseObject struct {
	Ref         string                 `json:"$ref,omitempty"`
	Description string                 `json:"description,omitempty"`
	Schema      *SchemaObject          `json:"schema,omitempty"`
	Headers     HeadersObject          `json:"headers,omitempty"`
	Examples    map[string]interface{} `json:"examples,omitempty"`
}

type SchemaObject struct {
	Ref        string                 `json:"$ref,omitempty"`
	Type       string                 `json:"type,omitempty"`
	Required   []string               `json:"required,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Items      *ReferenceObject       `json:"items,omitempty"`
}

type HeadersObject map[string]*HeaderObject

type HeaderObject struct {
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
}

type TagObject struct {
	Name         string                      `json:"name"`
	Description  string                      `json:"description,omitempty"`
	ExternalDocs ExternalDocumentationObject `json:"externalDocs,omitempty"`
}

type ExternalDocumentationObject struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

// type Property struct {
// 	Type        string `json:"type"`
// 	Format      string `json:"format,omitempty"`
// 	Description string `json:"description,omitempty"`
// }
