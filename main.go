package main

import (
  "strings"
  "net/url"
  "net/http"
  "encoding/gob"
  "os"
  "time"
  "golang.org/x/net/context"
  "golang.org/x/oauth2"
  "golang.org/x/oauth2/clientcredentials"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "github.com/gin-contrib/sessions"
  "github.com/gin-contrib/sessions/cookie"
  "github.com/gorilla/csrf"
  "github.com/gwatts/gin-adapter"
  "github.com/gofrs/uuid"
  oidc "github.com/coreos/go-oidc"
  "github.com/pborman/getopt"

  "github.com/charmixer/meui/app"
  "github.com/charmixer/meui/config"
  "github.com/charmixer/meui/environment"
  "github.com/charmixer/meui/utils"
  "github.com/charmixer/meui/controllers/callbacks"
  "github.com/charmixer/meui/controllers/profiles"
  "github.com/charmixer/meui/controllers/invites"
  "github.com/charmixer/meui/controllers/clients"
  "github.com/charmixer/meui/controllers/resourceservers"
  "github.com/charmixer/meui/controllers/access"
  "github.com/charmixer/meui/controllers/grant"
)

const appName = "meui"

var (
  logDebug int // Set to 1 to enable debug
  logFormat string // Current only supports default and json

  log *logrus.Logger

  appFields logrus.Fields
  sessionKeys environment.SessionKeys
)

func init() {
  log = logrus.New();

  err := config.InitConfigurations()
  if err != nil {
    log.Panic(err.Error())
    return
  }

  logDebug = config.GetInt("log.debug")
  logFormat = config.GetString("log.format")

  // We only have 2 log levels. Things developers care about (debug) and things the user of the app cares about (info)
  log = logrus.New();
  if logDebug == 1 {
    log.SetLevel(logrus.DebugLevel)
  } else {
    log.SetLevel(logrus.InfoLevel)
  }
  if logFormat == "json" {
    log.SetFormatter(&logrus.JSONFormatter{})
  }

  appFields = logrus.Fields{
    "appname": appName,
    "log.debug": logDebug,
    "log.format": logFormat,
  }

  sessionKeys = environment.SessionKeys{
    SessionAppStore: appName,
  }

  gob.Register(&oauth2.Token{}) // This is required to make session in meui able to persist tokens.
  gob.Register(&oidc.IDToken{})
  //gob.Register(&idp.Profile{})
  gob.Register(make(map[string][]string))
}

func main() {

  provider, err := oidc.NewProvider(context.Background(), config.GetString("hydra.public.url") + "/")
  if err != nil {
    logrus.WithFields(appFields).Panic("oidc.NewProvider" + err.Error())
    return
  }

  endpoint := provider.Endpoint()
  endpoint.AuthStyle = 2 // Force basic secret, so token exchange does not auto to post which we did not allow.


  // IdpApi needs to be able to act as an App using its client_id to bootstrap Authorization Code flow
  // Eg. Users accessing /me directly from browser.
  hydraConfig := &oauth2.Config{
    ClientID:     config.GetString("oauth2.client.id"),
    ClientSecret: config.GetString("oauth2.client.secret"),
    Endpoint:     endpoint,
    RedirectURL:  config.GetString("oauth2.callback"),
    Scopes:       config.GetStringSlice("oauth2.scopes.required"),
  }

  // IdpFe needs to be able as an App using client_id to access idp endpoints. Using client credentials flow
  idpConfig := &clientcredentials.Config{
    ClientID:  config.GetString("oauth2.client.id"),
    ClientSecret: config.GetString("oauth2.client.secret"),
    TokenURL: provider.Endpoint().TokenURL,
    Scopes: config.GetStringSlice("oauth2.scopes.required"),
    EndpointParams: url.Values{"audience": {"idp"}},
    AuthStyle: 2, // https://godoc.org/golang.org/x/oauth2#AuthStyle
  }

  aapConfig := &clientcredentials.Config{
    ClientID:  config.GetString("oauth2.client.id"),
    ClientSecret: config.GetString("oauth2.client.secret"),
    TokenURL: provider.Endpoint().TokenURL,
    Scopes: config.GetStringSlice("oauth2.scopes.required"),
    EndpointParams: url.Values{"audience": {"aap"}},
    AuthStyle: 2, // https://godoc.org/golang.org/x/oauth2#AuthStyle
  }

  // Setup app state variables. Can be used in handler functions by doing closures see exchangeAuthorizationCodeCallback
  env := &environment.State{
    SessionKeys: &sessionKeys,
    Provider: provider,
    HydraConfig: hydraConfig,
    IdpApiConfig: idpConfig,
    AapApiConfig: aapConfig,
  }

  optServe := getopt.BoolLong("serve", 0, "Serve application")
  optHelp := getopt.BoolLong("help", 0, "Help")
  getopt.Parse()

  if *optHelp {
    getopt.Usage()
    os.Exit(0)
  }

  if *optServe {
    serve(env)
  } else {
    getopt.Usage()
    os.Exit(0)
  }

}

func serve(env *environment.State) {
  r := gin.New() // Clean gin to take control with logging.
  r.Use(gin.Recovery())

  r.Use(requestId())
  r.Use(RequestLogger(env))

  store := cookie.NewStore([]byte(config.GetString("session.authKey")))
  // Ref: https://godoc.org/github.com/gin-gonic/contrib/sessions#Options
  store.Options(sessions.Options{
    MaxAge: 86400,
    Path: "/",
    Secure: true,
    HttpOnly: true,
  })
  r.Use(sessions.Sessions(env.SessionKeys.SessionAppStore, store))

  // Use CSRF on all meui forms.
  adapterCSRF := adapter.Wrap(csrf.Protect([]byte(config.GetString("csrf.authKey")), csrf.Secure(true)))
  // r.Use(adapterCSRF) // Do not use this as it will make csrf tokens for public files aswell which is just extra data going over the wire, no need for that.

  r.Static("/public", "public")
  r.LoadHTMLGlob("views/*")

  // Public endpoints
  ep := r.Group("/")
  ep.Use(adapterCSRF)
  {
    // Token exchange
    // FIXME: Must be public accessible until we figure out to enfore that only hydra client may make callbacks
    ep.GET("/callback", callbacks.ExchangeAuthorizationCodeCallback(env) )

    ep.GET("/profile", profiles.ShowPublicProfile(env) )

    ep.GET("/seeyoulater", profiles.ShowSeeYouLater(env) )
  }

  // Endpoints that require Authentication and Authorization
  ep = r.Group("/")
  ep.Use(adapterCSRF)
  ep.Use( AuthenticationRequired(env) )
  ep.Use( app.RequireIdentity(env) )
  {
    // Profile
    ep.GET(  "/",             profiles.ShowProfile(env) )
    ep.GET(  "/profile/edit", profiles.ShowProfileEdit(env) )
    ep.POST( "/profile/edit", profiles.SubmitProfileEdit(env) )

    ep.GET(  "/logout", profiles.ShowLogout(env) )

    // Invites
    ep.GET(  "/invites",      invites.ShowInvites(env) )
    ep.GET(  "/invites/send", invites.ShowInvitesSend(env) )
    ep.POST( "/invites/send", invites.SubmitInvitesSend(env) )
    ep.GET(  "/invite",       invites.ShowInvite(env) )
    ep.POST( "/invite",       invites.SubmitInvite(env) )

    // Clients
    ep.GET(  "/clients",        clients.ShowClients(env) )
    ep.GET(  "/clients/delete", clients.ShowClientDelete(env) )
    ep.POST( "/clients/delete", clients.SubmitClientDelete(env) )
    ep.GET(  "/client",         clients.ShowClient(env) )
    ep.POST( "/client",         clients.SubmitClient(env) )

    // Resource servers
    ep.GET(  "/resourceservers",        resourceservers.ShowResourceServers(env) )
    ep.GET(  "/resourceservers/delete", resourceservers.ShowResourceServerDelete(env) )
    ep.POST( "/resourceservers/delete", resourceservers.SubmitResourceServerDelete(env) )
    ep.GET(  "/resourceserver",         resourceservers.ShowResourceServer(env) )
    ep.POST( "/resourceserver",         resourceservers.SubmitResourceServer(env) )

    // Access
    ep.GET(  "/access",         access.ShowAccess(env))
    ep.GET(  "/access/grant",   grant.ShowGrants(env))
    ep.POST( "/access/grant",   grant.SubmitGrants(env))
    ep.GET(  "/access/new",     access.ShowAccessNew(env))
    ep.POST( "/access/new",     access.SubmitAccessNew(env))

  }

  r.RunTLS(":" + config.GetString("serve.public.port"), config.GetString("serve.tls.cert.path"), config.GetString("serve.tls.key.path"))
}

func RequestLogger(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    // Start timer
    start := time.Now()
    path := c.Request.URL.Path
    raw := c.Request.URL.RawQuery

    var requestId string = c.MustGet(environment.RequestIdKey).(string)
    requestLog := log.WithFields(appFields).WithFields(logrus.Fields{
      "request.id": requestId,
    })
    c.Set(environment.LogKey, requestLog)

    c.Next()

    // Stop timer
    stop := time.Now()
    latency := stop.Sub(start)

      ipData, err := utils.GetRequestIpData(c.Request)
      if err != nil {
        log.WithFields(appFields).WithFields(logrus.Fields{
          "func": "RequestLogger",
        }).Debug(err.Error())
      }

      forwardedForIpData, err := utils.GetForwardedForIpData(c.Request)
      if err != nil {
        log.WithFields(appFields).WithFields(logrus.Fields{
          "func": "RequestLogger",
        }).Debug(err.Error())
      }

    method := c.Request.Method
    statusCode := c.Writer.Status()
    errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

    bodySize := c.Writer.Size()

      var fullpath string = path
    if raw != "" {
    fullpath = path + "?" + raw
    }

    // if public data is requested successfully, then dont log it since its just spam when debugging
    if strings.Contains(path, "/public/") && ( statusCode == http.StatusOK || statusCode == http.StatusNotModified ) {
     return
    }

    log.WithFields(appFields).WithFields(logrus.Fields{
      "latency": latency,
      "forwarded_for.ip": forwardedForIpData.Ip,
      "forwarded_for.port": forwardedForIpData.Port,
      "ip": ipData.Ip,
      "port": ipData.Port,
      "method": method,
      "status": statusCode,
      "error": errorMessage,
      "body_size": bodySize,
      "path": fullpath,
      "request.id": requestId,
    }).Info("")
  }
  return gin.HandlerFunc(fn)
}

// # Authentication and Authorization
// Gin middleware to secure idp fe endpoints using oauth2.
//
// ## QTNA - Questions that need answering before granting access to a protected resource
// 1. Is the user or client authenticated? Answered by the process of obtaining an access token.
// 2. Is the access token expired?
// 3. Is the access token granted the required scopes?
// 4. Is the user or client giving the grants in the access token authorized to operate the scopes granted?
// 5. Is the access token revoked?

func AuthenticationRequired(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "AuthenticationRequired",
    })

    session := sessions.Default(c)

    // Authenticate by looking for valid access token
    var token *oauth2.Token

    token = authenticateWithBearer(c.Request)
    if token != nil {
      log = log.WithFields(logrus.Fields{"authorization": "bearer"})
      log.Debug("Access token found")
    } else {

      token = authenticateWithSession(session, environment.SessionTokenKey)
      if token != nil {
        log = log.WithFields(logrus.Fields{"authorization": "session"})
        log.Debug("Access token found")
      }

    }

    if token != nil {

      tokenSource := env.HydraConfig.TokenSource(oauth2.NoContext, token)
      newToken, err := tokenSource.Token()
      if err != nil {
        log.Debug(err.Error())
        c.AbortWithStatus(http.StatusInternalServerError)
        return
      }

      if newToken.AccessToken != token.AccessToken {
        log.Debug("Access token refreshed")
        token = newToken
      }

      // See #2 of QTNA
      // https://godoc.org/golang.org/x/oauth2#Token.Valid
      if token.Valid() == true {
        log.Debug("Access token valid")

        // See #5 of QTNA
        log.WithFields(logrus.Fields{"fixme": 1, "qtna": 5}).Debug("Missing check against token-revoked-list to check if token is revoked") // Call token revoked list to check if token is revoked.

        session.Set(environment.SessionTokenKey, token)
        err = session.Save()
        if err != nil {
          log.Debug(err.Error())
          c.AbortWithStatus(http.StatusInternalServerError)
          return
        }

        log.Debug("Authenticated")
        c.Next()
        return
      }

    }

    // Deny by default
    log.Debug("Unauthorized")

    initUrl, err := app.StartAuthenticationSession(env, c, log)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }
    c.Redirect(http.StatusFound, initUrl.String())
    c.Abort()
    return
  }
  return gin.HandlerFunc(fn)
}

/*func AuthorizationRequired(env *environment.State, requiredScopes ...string) gin.HandlerFunc {
  fn := func(c *gin.Context) {
    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "AuthorizationRequired",
    })

    strRequiredScopes := strings.Join(requiredScopes, " ")
    log.WithFields(logrus.Fields{"scope": strRequiredScopes}).Debug("Required Scopes");

    aapClient := app.AapClientUsingClientCredentials(env, c)

    judgeRequest := []aap.ReadEntitiesJudgeRequest{ {
      Publisher: "a73b547b-f26d-487b-9e3a-2574fe3403fe", // Resource Server. For IdpUI this is IDP, FIXME: We should be able to specify audience instead of id (as thirdparty might not know id)
      Owners: []string{config.GetString("oauth2.client.id")}, // meui client
      Scopes: requiredScopes,
    }}
    status, responses, err := aap.ReadEntitiesJudge(aapClient, config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.entities.judge"), judgeRequest)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    if status == 200 {

      // QTNA answered by app judge endpoint
      // #3 - Access token granted required scopes? (hydra token introspect)
      // #4 - User or client in access token authorized to execute the granted scopes?
      var verdict aap.ReadEntitiesJudgeResponse
      status, restErr := bulky.Unmarshal(0, responses, &verdict)
      if restErr != nil {
        log.Debug("Unmarshal failed")
        c.AbortWithStatus(http.StatusInternalServerError)
        return
      }

      if status == 200 {

        if verdict.Granted == true {
          log.Debug("Authorized")
          c.Next()
          return
        }

      }

    }

    // Deny by Default
    log.Debug("Forbidden")
    c.AbortWithStatus(http.StatusForbidden)
    return
  }
  return gin.HandlerFunc(fn)
}*/

func authenticateWithBearer(req *http.Request) (*oauth2.Token) {
  auth := req.Header.Get("Authorization")
  split := strings.SplitN(auth, " ", 2)
  if len(split) == 2 || strings.EqualFold(split[0], "bearer") {
    token := &oauth2.Token{
      AccessToken: split[1],
      TokenType: split[0],
    }
    return token
  }
  return nil
}

func authenticateWithSession(session sessions.Session, tokenKey string) (*oauth2.Token) {
  v := session.Get(tokenKey)
  if v != nil {
    return v.(*oauth2.Token)
  }
  return nil
}

func requestId() gin.HandlerFunc {
  return func(c *gin.Context) {
  // Check for incoming header, use it if exists
  requestID := c.Request.Header.Get("X-Request-Id")

  // Create request id with UUID4
  if requestID == "" {
  uuid4, _ := uuid.NewV4()
  requestID = uuid4.String()
  }

  // Expose it for use in the application
  c.Set("RequestId", requestID)

  // Set X-Request-Id header
  c.Writer.Header().Set("X-Request-Id", requestID)
  c.Next()
  }
}
