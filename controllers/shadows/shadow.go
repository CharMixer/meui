package shadows

import (
  "net/http"
  "net/url"
  "time"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  //"github.com/gin-contrib/sessions"
  //idp "github.com/charmixer/idp/client"
  aap "github.com/charmixer/aap/client"

  "github.com/charmixer/meui/app"
  "github.com/charmixer/meui/config"
  "github.com/charmixer/meui/environment"

  f "github.com/go-playground/form"

  bulky "github.com/charmixer/bulky/client"

  "github.com/gorilla/csrf"
)

type formInput struct {
  Identity string
  Role string
  StartDate string
  EndDate string
}

func ShowShadow(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    role, roleExists := c.GetQuery("role")

    if !roleExists {
      log.Debug("Missing role in query")
      c.AbortWithStatus(404)
      return
    }

    c.HTML(http.StatusOK, "shadow.html", gin.H{
      "title": "Create new shadow",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "role": role,
      "id": identity.Id,
      "user": identity.Username,
      "name": identity.Name,
    })

  }
  return gin.HandlerFunc(fn)
}

func SubmitShadow(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)

    var form formInput
    c.Request.ParseForm()

    decoder := f.NewDecoder()

    // must pass a pointer
    err := decoder.Decode(&form, c.Request.Form)
    if err != nil {
      log.Panic(err)
      c.AbortWithStatus(404)
      return
    }

    aapClient := app.AapClientUsingAuthorizationCode(env, c)

    var createShadowsRequests []aap.CreateShadowsRequest

    layout := "2006-01-02"

    var nbf int64
    if form.StartDate != "" {
      nbfTime, err := time.Parse(layout, form.StartDate)
      if err != nil {
         panic(err)
      }

      nbf = nbfTime.Unix()
    }

    var exp int64
    if form.EndDate != "" {
      expTime, err := time.Parse(layout, form.EndDate)
      if err != nil {
         panic(err)
      }

      exp = expTime.Unix()
    }

    createShadowsRequests = append(createShadowsRequests, aap.CreateShadowsRequest{
      Identity: form.Identity,
      Shadow: form.Role,
      NotBefore: nbf,
      Expire: exp,
    })

    callUrl := config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.shadows.collection")
    httpStatus, responses, err := aap.CreateShadows(aapClient, callUrl, createShadowsRequests)

    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(404)
      return
    }

    if httpStatus != 200 {
      log.Debug("Failed to get 200 from " + callUrl);
      c.AbortWithStatus(404)
      return
    }

    var createShadowsResponse aap.CreateShadowsResponse
    restStatus, restErr := bulky.Unmarshal(0, responses, &createShadowsResponse)

    if restErr != nil {
      for _,e := range restErr {
        // TODO show user somehow
        log.Debug("Rest error: " + e.Error)
      }
      c.AbortWithStatus(404)
      return
    }

    successUrl, err := url.Parse(config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.shadows.collection"))
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    _successUrl := *successUrl
    q := _successUrl.Query()
    q.Add("role", form.Role)
    _successUrl.RawQuery = q.Encode()

    if restStatus == 200 {
      c.Redirect(http.StatusFound, _successUrl.String())
      c.Abort()
      return
    }

    failureUrl, err := url.Parse(config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.shadow"))
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    _failureUrl := *failureUrl
    q = _failureUrl.Query()
    q.Add("role", form.Role)
    _failureUrl.RawQuery = q.Encode()

    c.Redirect(http.StatusFound, _failureUrl.String())
  }
  return gin.HandlerFunc(fn)
}
