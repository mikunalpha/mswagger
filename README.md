# mswagger
Based on [yvasiyarov/swagger](https://github.com/yvasiyarov/swagger) repository.

## Todo
1. ~~Support to parse vendor folder.~~
2. ~~Generate swagger 2.0 json file.~~
3. ~~Fix mappings of some field type with swagger-ui.~~

## Usage
Comments in main.go
```go
// @Version 1.0.0
// @Title Backend API
// @Description API usually works as expected. But sometimes its not true.
// @BasePath /
// @Schemes http,https
// @ContactName Abcd
// @ContactEmail abce@email.com
// @ContactURL http://someurl.oxox
// @TermsOfServiceUrl http://someurl.oxox
// @LicenseName MIT
// @LicenseURL https://en.wikipedia.org/wiki/MIT_License
```
Comments for API handleFunc
```go
type User struct {
  Id   uint64 `json:"id"`
  Name string `json:"name"`
}

type UsersResponse struct {
  Data []Users `json:"users"`
}

type Error struct {
  Code string `json:"code"`
  Msg  string `json:"msg"`
}

type ErrorResponse struct {
  ErrorInfo Error `json:"error"`
}

// @Title Get user list of a group.
// @Resource users "Normal user of this system"
// @Description Get users related to a specific group.
// @Param  group_id  path  int  true  "Id of a specific group."
// @Success  200  {object}  UsersResponse  "UsersResponse JSON"
// @Failure  400  {object}  ErrorResponse  "ErrorResponse JSON"
// @Produce json
// @Router /api/group/{group_id}/users [get]
func GetGroupUsers() {
  // ...
}

// @Title Get user list of a group.
// @Resource users
// @Description Create a new user.
// @Param  user  body  User  true  "Info of a user."
// @Success  200  {object}  User           "UsersResponse JSON"
// @Failure  400  {object}  ErrorResponse  "ErrorResponse JSON"
// @Produce json
// @Router /api/user [post]
func PostUser() {
  // ...
}
```
Code in main.go
```go
import "github.com/mikunalpha/mswagger"

// ...
  params := mswagger.Params{
    OutputPath: "./swagger.json",
  }

  if err := mswagger.Run(params); err != nil {
    fmt.Println(err)
  }
// ...
```
