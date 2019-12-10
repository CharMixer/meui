package roles

import (
  "net/http"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  //"github.com/gin-contrib/sessions"
  idp "github.com/opensentry/idp/client"

  "github.com/opensentry/meui/app"
  "github.com/opensentry/meui/config"
  "github.com/opensentry/meui/environment"
  "github.com/opensentry/meui/utils"

  f "github.com/go-playground/form"

  bulky "github.com/charmixer/bulky/client"

  "github.com/gorilla/csrf"
)

type formRoleDelete struct {
  Id          string
  Terms       string `form:"risk_accepted"`
}

func ShowRoleDelete(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowRoleDelete",
    })

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    roleToDeleteId := c.Query("id")
    if roleToDeleteId == "" {
      log.Debug("Missing id")
      c.AbortWithStatus(http.StatusNotFound)
      return
    }

    var role *idp.Role
    idpClient := app.IdpClientUsingAuthorizationCode(env, c)
    readRequest := []idp.ReadRolesRequest{ {Id: roleToDeleteId} }
    status, responses, err := idp.ReadRoles(idpClient, config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.roles.collection"), readRequest)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    if status == 200 {

      var resp idp.ReadRolesResponse
      status, _ = bulky.Unmarshal(0, responses, &resp)
      if status == 200 {
        role = &resp[0]
      }

    }

    if role == nil {
      log.Debug("Role does not exist")
      c.AbortWithStatus(http.StatusNotFound)
      return
    }

    submitUrl, err := utils.FetchSubmitUrlFromRequest(c.Request, nil)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }


    c.HTML(http.StatusOK, "roledelete.html", gin.H{
      "title": "Delete role",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "submitUrl": submitUrl,
      "role": role,
      "provider": config.GetString("provider.name"),
    })

  }
  return gin.HandlerFunc(fn)
}

func SubmitRoleDelete(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "SubmitRoleDelete",
    })

    var form formRoleDelete
    c.Request.ParseForm()

    decoder := f.NewDecoder()

    // must pass a pointer
    err := decoder.Decode(&form, c.Request.Form)
    if err != nil {
      log.Panic(err)
      c.AbortWithStatus(404)
      return
    }

    if form.Terms != "accept" {
      c.Redirect(http.StatusFound, "/roles/delete?id="+form.Id)
      c.Abort()
      return
    }

    idpClient := app.IdpClientUsingAuthorizationCode(env, c)

    var deleteRolesRequests []idp.DeleteRolesRequest

    deleteRolesRequests = append(deleteRolesRequests, idp.DeleteRolesRequest{
      Id: form.Id,
    })

    url := config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.roles.collection")
    httpStatus, responses, err := idp.DeleteRoles(idpClient, url, deleteRolesRequests)

    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(404)
      return
    }

    if httpStatus != 200 {
      log.Debug("Failed to get 200 from " + url);
      c.AbortWithStatus(404)
      return
    }

    var deleteRolesResponse idp.DeleteRolesResponse
    restStatus, restErr := bulky.Unmarshal(0, responses, &deleteRolesResponse)

    if restErr != nil {
      for _,e := range restErr {
        // TODO show user somehow
        log.Debug("Rest error: " + e.Error)
      }
      c.AbortWithStatus(404)
      return
    }

    if restStatus == 200 {
      c.Redirect(http.StatusFound, "/roles")
      c.Abort()
      return
    }

    c.Redirect(http.StatusFound, "/roles/delete")
  }
  return gin.HandlerFunc(fn)
}
