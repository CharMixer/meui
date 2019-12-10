package invites

import (
  "strings"
  "time"
  "net/http"
  "reflect"
  "gopkg.in/go-playground/validator.v9"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "github.com/gorilla/csrf"
  "github.com/gin-contrib/sessions"

  "github.com/go-playground/form"

  idp "github.com/opensentry/idp/client"

  "github.com/opensentry/meui/app"
  "github.com/opensentry/meui/config"
  "github.com/opensentry/meui/environment"
  "github.com/opensentry/meui/utils"
  "github.com/opensentry/meui/validators"
)

type inviteForm struct {
  Email     string `binding:"required" validate:"required,email"`
  Username  string
  ExpiresAt string
}

func ShowInvite(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowInvite",
    })

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    session := sessions.Default(c)

    // Retain the values that was submittet, except passwords ?!
    var username string
    var email string
    rf := session.Flashes("invite.fields")
    if len(rf) > 0 {
      fields := rf[0].(map[string][]string)
      for k, v := range fields {
        if k == "Email" && len(v) > 0 {
          email = strings.Join(v, ", ")
        }

        if k == "Username" && len(v) > 0 {
          username = strings.Join(v, ", ")
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

        if k == "Email" && len(v) > 0 {
          errorEmail = strings.Join(v, ", ")
        }

        if k == "Username" && len(v) > 0 {
          errorUsername = strings.Join(v, ", ")
        }

      }
    }

    c.HTML(http.StatusOK, "invite.html", gin.H{
      "title": "Invite",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "provider": config.GetString("provider.name"),
      "id": identity.Id,
      "user": identity.Username,
      "name": identity.Name,
      "Username": username,
      "Email": email,
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

    c.Request.ParseForm()

    var input inviteForm
    err := form.NewDecoder().Decode(&input, c.Request.Form)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusBadRequest)
      return
    }

    log.Debug(input)

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    session := sessions.Default(c)

    // Save values if submit fails
    fields := make(map[string][]string)
    fields["Email"] = append(fields["Email"], input.Email)
    fields["Username"] = append(fields["Username"], input.Username)

    session.AddFlash(fields, "invite.fields")
    err = session.Save()
    if err != nil {
      log.Debug(err.Error())
    }

    errors := make(map[string][]string)
    validate := validator.New()
    validate.RegisterValidation("notblank", validators.NotBlank)
    err = validate.Struct(input)
    if err != nil {

      // Validation syntax is invalid
      if err,ok := err.(*validator.InvalidValidationError); ok{
        log.Debug(err.Error())
        c.AbortWithStatus(http.StatusInternalServerError)
        return
      }

      reflected := reflect.ValueOf(input) // Use reflector to reverse engineer struct
      for _, err := range err.(validator.ValidationErrors){

        // Attempt to find field by name and get json tag name
        field,_ := reflected.Type().FieldByName(err.StructField())
        var name string

        // If form tag doesn't exist, use lower case of name
        if name = field.Tag.Get("form"); name == ""{
          name = err.StructField()
        }

        switch err.Tag() {
        case "required":
            errors[name] = append(errors[name], "Required")
            break
        case "email":
            errors[name] = append(errors[name], "Invalid E-mail")
            break
        case "eqfield":
            errors[name] = append(errors[name], "Should be equal to the "+err.Param())
            break
        case "notblank":
          errors[name] = append(errors[name], "Blank not allowed")
          break
        default:
            errors[name] = append(errors[name], "Invalid")
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

    var expiresAt int64 = 0
    if input.ExpiresAt != "" {
      expiresAtTime, err := time.Parse("2006-01-02", input.ExpiresAt)
      if err != nil {
        log.Debug(err.Error())
        c.AbortWithStatus(http.StatusInternalServerError)
        return
      }
      expiresAt = expiresAtTime.Unix()
    }

    inviteRequest := []idp.CreateInvitesRequest{{
      Email: input.Email,
      Username: input.Username,
      ExpiresAt: expiresAt,
    }}
    status, invite, err := idp.CreateInvites(idpClient, config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.invites.collection"), inviteRequest)
    if err != nil {
      log.WithFields(logrus.Fields{ "email":input.Email, "username":input.Username, "exp":input.ExpiresAt }).Debug("Invite failed")
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

      redirectTo := config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.invites.collection")
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
