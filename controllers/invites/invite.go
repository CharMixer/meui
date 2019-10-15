package invites

import (
  "strings"
  "net/http"
  "reflect"
  "strconv"
  "gopkg.in/go-playground/validator.v9"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "github.com/gorilla/csrf"
  "github.com/gin-contrib/sessions"
  idp "github.com/charmixer/idp/client"

  "github.com/charmixer/meui/app"
  "github.com/charmixer/meui/config"
  "github.com/charmixer/meui/environment"
  "github.com/charmixer/meui/utils"
  "github.com/charmixer/meui/validators"
)

type inviteForm struct {
  Email     string `form:"email" binding:"required" validate:"required,email"`
  Username  string `form:"username"`
  ExpiresAt int64 `form:"exp" binding:"numeric" validate:"numeric"`
}

func ShowInvite(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowInvite",
    })

    identity := app.RequireIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    session := sessions.Default(c)

    // Retain the values that was submittet, except passwords ?!
    var username string
    var email string
    var exp string
    rf := session.Flashes("invite.fields")
    if len(rf) > 0 {
      fields := rf[0].(map[string][]string)
      for k, v := range fields {
        if k == "email" && len(v) > 0 {
          email = strings.Join(v, ", ")
        }

        if k == "username" && len(v) > 0 {
          username = strings.Join(v, ", ")
        }

        if k == "exp" && len(v) > 0 {
          exp = strings.Join(v, ", ")
        }
      }
    }

    errors := session.Flashes("invite.errors")
    err := session.Save() // Remove flashes read, and save submit fields
    if err != nil {
      log.Debug(err.Error())
    }

    var errorEmail string
    var errorUsername string
    var errorExp string

    if len(errors) > 0 {
      errorsMap := errors[0].(map[string][]string)
      for k, v := range errorsMap {

        if k == "email" && len(v) > 0 {
          errorEmail = strings.Join(v, ", ")
        }

        if k == "username" && len(v) > 0 {
          errorUsername = strings.Join(v, ", ")
        }

        if k == "exp" && len(v) > 0 {
          errorExp = strings.Join(v, ", ")
        }
      }
    }

    c.HTML(http.StatusOK, "invite.html", gin.H{
      "title": "Invite",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "username": username,
      "email": email,
      "exp": exp,
      "errorEmail": errorEmail,
      "errorUsername": errorUsername,
      "errorExp": errorExp,
    })
  }
  return gin.HandlerFunc(fn)
}

func SubmitInvite(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "SubmitInvite",
    })

    var form inviteForm
    err := c.Bind(&form)
    if err != nil {
      c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
      c.Abort()
      return
    }

    identity := app.RequireIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    session := sessions.Default(c)

    // Save values if submit fails
    fields := make(map[string][]string)
    fields["email"] = append(fields["email"], form.Email)
    fields["username"] = append(fields["username"], form.Username)
    fields["exp"] = append(fields["exp"], strconv.FormatInt(form.ExpiresAt, 10))

    session.AddFlash(fields, "invite.fields")
    err = session.Save()
    if err != nil {
      log.Debug(err.Error())
    }

    errors := make(map[string][]string)
    validate := validator.New()
    validate.RegisterValidation("notblank", validators.NotBlank)
    err = validate.Struct(form)
    if err != nil {

      // Validation syntax is invalid
      if err,ok := err.(*validator.InvalidValidationError); ok{
        log.Debug(err.Error())
        c.AbortWithStatus(http.StatusInternalServerError)
        return
      }

      reflected := reflect.ValueOf(form) // Use reflector to reverse engineer struct
      for _, err := range err.(validator.ValidationErrors){

        // Attempt to find field by name and get json tag name
        field,_ := reflected.Type().FieldByName(err.StructField())
        var name string

        // If form tag doesn't exist, use lower case of name
        if name = field.Tag.Get("form"); name == ""{
          name = strings.ToLower(err.StructField())
        }

        switch err.Tag() {
        case "required":
            errors[name] = append(errors[name], "Field is required")
            break
        case "email":
            errors[name] = append(errors[name], "Field must be a valid email")
            break
        case "eqfield":
            errors[name] = append(errors[name], "Field should be equal to the "+err.Param())
            break
        case "notblank":
          errors[name] = append(errors[name], "Field is not allowed to be blank")
          break
        default:
            errors[name] = append(errors[name], "Field is invalid")
            break
        }
      }

    }

    if len(errors) > 0 {
      session.AddFlash(errors, "invite.errors")
      err = session.Save()
      if err != nil {
        log.Debug(err.Error())
      }

      submitUrl, err := utils.FetchSubmitUrlFromRequest(c.Request, nil)
      if err != nil {
        log.Debug(err.Error())
        c.AbortWithStatus(http.StatusInternalServerError)
        return
      }
      log.WithFields(logrus.Fields{"redirect_to": submitUrl}).Debug("Redirecting")
      c.Redirect(http.StatusFound, submitUrl)
      c.Abort()
      return
    }

    idpClient := app.IdpClientUsingAuthorizationCode(env, c)

    inviteRequest := []idp.CreateInvitesRequest{{
      Email: form.Email,
      Username: form.Username,
      ExpiresAt: form.ExpiresAt,
    }}
    status, invite, err := idp.CreateInvites(idpClient, config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.invites.collection"), inviteRequest)
    if err != nil {
      log.WithFields(logrus.Fields{ "email":form.Email, "username":form.Username, "exp":form.ExpiresAt }).Debug("Invite failed")
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    if status == 200 && invite != nil {

      // Cleanup session
      session.Delete("invite.fields")
      session.Delete("invite.errors")
      err = session.Save()
      if err != nil {
        log.Debug(err.Error())
      }

      redirectTo := config.GetString("idpui.public.url") + config.GetString("idpui.public.endpoints.invites.collection")
      log.WithFields(logrus.Fields{"redirect_to": redirectTo}).Debug("Redirecting")

      c.Redirect(http.StatusFound, redirectTo)
      c.Abort()
      return
    }

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
