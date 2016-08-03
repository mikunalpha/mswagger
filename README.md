# mswagger
Based on [yvasiyarov/swagger](https://github.com/yvasiyarov/swagger) repository.
 
## About 
1. Support to parse vendor folder.
2. Generate swagger 2.0 json file.
3. Fix mappings of some field type with swagger-ui.

## Usage
Comments in main.go
```go
// @Version 1.0.0
// @Title Minapp
// @Description API usually works as expected. But sometimes its not true.
// @BasePath /
// @Schemes http, https
// @ContactName Abcd
// @ContactEmail abce@email.com
// @ContactURL http://someurl.oxox
// @TermsOfServiceUrl http://someurl.oxox
// @LicenseName MIT
// @LicenseURL https://en.wikipedia.org/wiki/MIT_License
```
Comments for API handleFunc
```go
type EmptyResponse struct {}

type Error struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

type ErrorResponse struct {
	Error Error `json:"error"`
}

// @Title Get user list of a group.
// @Resource users
// @Description Get users related to a specific group.
// @Param  group_id  path  int  true  "Test token."
// @Success  200  {object}  EmptyResponse  "EmptyResponse JSON"
// @Failure  400  {object}  ErrorResponse  "ErrorResponse JSON"
// @Produce json
// @Router /api/group/{group_id} [get]
func GetUsers() {
  // ...
}
```
Code in main.go
```go
import "github.com/mikunalpha/mswagger"

// ...
  folderPath := "./" // Default is "swagger-ui"
  if err := mswagger.Run(path); err != nil {
    fmt.Println(err)
  }
// ...
```
