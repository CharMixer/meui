package roles

import (
  "net/url"
  "net/http"
  "sort"
  "fmt"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  //"github.com/gin-contrib/sessions"
  idp "github.com/charmixer/idp/client"

  "github.com/charmixer/meui/app"
  "github.com/charmixer/meui/config"
  "github.com/charmixer/meui/environment"

  bulky "github.com/charmixer/bulky/client"
)

type RoleTemplate struct {
  Id string
  Name string
  Description string
  Secret string
  GrantsUrl string
  SubscriptionsUrl string
  DeleteUrl string
  ShadowsUrl string
}

func ShowRoles(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowClients",
    })

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    idpClient := app.IdpClientUsingAuthorizationCode(env, c)

    status, responses, err := idp.ReadRoles(idpClient, config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.roles.collection"), nil)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    if status != 200 {
      log.Debug(fmt.Sprintf("Failed to get OK response, got %d", status))
      c.AbortWithStatus(http.StatusNotFound)
      return
    }

    grantsUrl, err := url.Parse(config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.access.grant"))
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    deleteUrl, err := url.Parse(config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.roles.delete"))
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    shadowsUrl, err := url.Parse(config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.shadows.collection"))
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    var uiCreatedRoles []RoleTemplate

    var roles idp.ReadRolesResponse
    status, _ = bulky.Unmarshal(0, responses, &roles)
    if status == 200 {

      for _, role := range roles {

        _grantsUrl := *grantsUrl
        q := _grantsUrl.Query()
        q.Add("receiver", role.Id)
        _grantsUrl.RawQuery = q.Encode()

        _deleteUrl := *deleteUrl
        q = _deleteUrl.Query()
        q.Add("id", role.Id)
        _deleteUrl.RawQuery = q.Encode()

        _shadowsUrl := *shadowsUrl
        q = _shadowsUrl.Query()
        q.Add("role", role.Id)
        _shadowsUrl.RawQuery = q.Encode()

        uiRole := RoleTemplate{
          Id:               role.Id,
          Name:             role.Name,
          Description:      role.Description,
          GrantsUrl:        _grantsUrl.String(),
          DeleteUrl:        _deleteUrl.String(),
          ShadowsUrl:       _shadowsUrl.String(),
        }
        uiCreatedRoles = append(uiCreatedRoles, uiRole)

      }

    }

    sort.Slice(uiCreatedRoles, func(i, j int) bool {
      return uiCreatedRoles[i].Name > uiCreatedRoles[j].Name
    })

    c.HTML(http.StatusOK, "roles.html", gin.H{
      "title": "Roles",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      "provider": config.GetString("provider.name"),
      "id": identity.Id,
      "user": identity.Username,
      "name": identity.Name,
      "created": uiCreatedRoles,
    })
  }
  return gin.HandlerFunc(fn)
}
