package clients

import (
  "net/http"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "github.com/gorilla/csrf"
  idp "github.com/opensentry/idp/client"

  "github.com/go-playground/form"

  bulky "github.com/charmixer/bulky/client"

  "github.com/opensentry/meui/app"
  "github.com/opensentry/meui/config"
  "github.com/opensentry/meui/environment"
  "github.com/opensentry/meui/utils"
)

type formEditClient struct {
  Name                    string   `validate:"required,notblank"`
  Description             string   `validate:"required,notblank"`
  RedirectUris            []string
  PostLogoutRedirectUris  []string
  TokenEndpointAuthMethod string
  GrantTypes              []string
  ResponseTypes           []string
  IsPublic                string
}

func ShowClientEdit(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    clientIdToEdit := c.Query("id")
    if clientIdToEdit == "" {
      log.Debug("Missing id")
      c.AbortWithStatus(http.StatusNotFound)
      return
    }

    idpClient := app.IdpClientUsingAuthorizationCode(env, c)

    url := config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.clients.collection")
    clients := []idp.ReadClientsRequest{ {Id: clientIdToEdit} }
    status, responses, err := idp.ReadClients(idpClient, url, clients)

    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    if status != 200 {
      log.Debug("Expected to get 200 but got "+string(status)+" from GET " + url)
      c.AbortWithStatus(http.StatusNotFound)
      return
    }

    var readClientsResponse idp.ReadClientsResponse
    status, restErr := bulky.Unmarshal(0, responses, &readClientsResponse)

    if status != 200 {
      log.Debug("Expected to get virtual status 200 but got "+string(status)+" from GET " + url)
      c.AbortWithStatus(http.StatusNotFound)
      return
    }

    if restErr != nil {
      for _,e := range restErr {
        log.Debug(e)
      }
      c.AbortWithStatus(http.StatusNotFound)
      return
    }

    readClient := readClientsResponse[0]

    var form = formEditClient{
      Name:                    readClient.Name,
      Description:             readClient.Description,
      RedirectUris:            readClient.RedirectUris,
      PostLogoutRedirectUris:  readClient.PostLogoutRedirectUris,
      TokenEndpointAuthMethod: readClient.TokenEndpointAuthMethod,
      GrantTypes:              readClient.GrantTypes,
      ResponseTypes:           readClient.ResponseTypes,
    }

    c.HTML(http.StatusOK, "editclient.html", gin.H{
      "title": "Client",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "provider": config.GetString("provider.name"),
      "id": identity.Id,
      "user": identity.Username,
      "name": identity.Name,
      "form": form,
      "tokenEndpointAuthMethods": fetchTokenEndpointAuthMethods(),
      "grantTypes": fetchGrantTypes(),
      "responseTypes": fetchResponseTypes(),
      "formUrl": config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.clients.edit") + "?id=" + clientIdToEdit,
    })
  }
  return gin.HandlerFunc(fn)
}

func fetchTokenEndpointAuthMethods() ([]string) {
  return []string{"none", "client_secret_post", "client_secret_basic", "private_key_jwt"}
}

func fetchGrantTypes() ([]string) {
  return []string{"authorization_code", "implicit", "password", "client_credentials", "device_code", "refresh_token"}
}

func fetchResponseTypes() ([]string) {
  return []string{"code", "token"}
}

func SubmitClientEdit(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)

    var input formEditClient
    c.Request.ParseForm()

    decoder := form.NewDecoder()

    // must pass a pointer
    err := decoder.Decode(&input, c.Request.Form)
    if err != nil {
      log.Panic(err)
      c.AbortWithStatus(400)
      return
    }

    //var isPublic bool = false
    //if input.IsPublic == "on" {
      //isPublic = true
    //}

    var redirectUris []string
    for _,uri := range input.RedirectUris {
      if uri == "" {
        continue
      }

      redirectUris = append(redirectUris, uri)
    }

    var postLogoutRedirectUris []string
    for _,uri := range input.PostLogoutRedirectUris {
      if uri == "" {
        continue
      }

      postLogoutRedirectUris = append(postLogoutRedirectUris, uri)
    }

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    clientIdToEdit := c.Query("id")
    if clientIdToEdit == "" {
      log.Debug("Missing id")
      c.AbortWithStatus(http.StatusNotFound)
      return
    }

    idpClient := app.IdpClientUsingAuthorizationCode(env, c)

    status, responses, err := idp.UpdateClients(idpClient, config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.clients.collection"), []idp.UpdateClientsRequest{
      {
        Id:                      clientIdToEdit,
        Name:                    input.Name,
        Description:             input.Description,
        ResponseTypes:           input.ResponseTypes,
        GrantTypes:              input.GrantTypes,
        RedirectUris:            redirectUris,
        PostLogoutRedirectUris:  postLogoutRedirectUris,
        TokenEndpointAuthMethod: input.TokenEndpointAuthMethod,
        // IsPublic:                isPublic,
      },
    })
    if err != nil || status != 200 {
      log.Debug("Client update failed")
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    var updateClientResponse idp.UpdateClientsResponse
    bulky.Unmarshal(0, responses, &updateClientResponse)

    // Deny by default. Failed to fill in the form correctly.
    submitUrl, err := utils.FetchSubmitUrlFromRequest(c.Request, nil)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }
    log.WithFields(logrus.Fields{"redirect_to": submitUrl}).Debug("Redirecting")
    c.Redirect(http.StatusFound, submitUrl)
    c.Abort()
  }
  return gin.HandlerFunc(fn)
}
