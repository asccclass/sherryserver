# OAuth 服務

## Use PEM Keys

* Generate private and public RSA keys
```
openssl genrsa -out private_key.pem 2048
openssl rsa -pubout -in private_key.pem -out public_key.pem
```

* Use it at go code
```
var (
   publicKey  *rsa.PublicKey
   privateKey *rsa.PrivateKey
)

// Load both public (verify) and private (sign) RSA keys
func init() {
   publicKeyData, err := os.ReadFile("./public_key.pem")
   if err != nil {
      log.Fatal(err)
   }
   publicKey, err = jwt.ParseRSAPublicKeyFromPEM(publicKeyData)
   if err != nil {
      log.Fatal(err)
   }

   privateKeyData, err := os.ReadFile("./private_key.pem")
   if err != nil {
      log.Fatal(err)
   }
   privateKey, err = jwt.ParseRSAPrivateKeyFromPEM(privateKeyData)
   if err != nil {
      log.Fatal(err)
   }
}

## Step 2. Issuer
```
func main() {
	e := echo.New()
	e.Use(middleware.Logger())

	e.POST("/login", login)

	e.Start("127.0.0.1:4242")
}

// We can add custom claims here
type jwtClaims struct {
	jwt.RegisteredClaims
}

func login(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	// TODO: implement real auth by checking user in the database
	if username != "package" || password != "main" {
		return echo.ErrUnauthorized
	}

	// Set expiration time (1h)
	claims := &jwtClaims{
		jwt.RegisteredClaims{
			Subject:   username,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	t, err := token.SignedString(privateKey)
	if err != nil {
		return echo.ErrInternalServerError
	}

	return c.JSON(http.StatusOK, echo.Map{
		"token": t,
	})
}
```

## Step 3. Middleware
```
func main() {
	e := echo.New()
	e.Use(middleware.Logger())

	config := echojwt.Config{
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return new(jwtClaims)
		},
		SigningKey:    publicKey,
		SigningMethod: jwt.SigningMethodRS256.Name,
	}

	g := e.Group("/api")
	g.Use(echojwt.WithConfig(config))
	g.GET("/greet", greet)

	e.Start("127.0.0.1:4242")
}

type jwtClaims struct {
	jwt.RegisteredClaims
}

func greet(c echo.Context) error {
	user := c.Get("user").(*jwt.Token)
	claims := user.Claims.(*jwtClaims)
	sub := claims.Subject
	return c.String(http.StatusOK, fmt.Sprintf("hi %s!", sub))
}
```

## 參考資料
* https://packagemain.tech/p/json-web-tokens-in-go?utm_source=christophberger&utm_medium=email&utm_campaign=2025-06-01-stop-splitting-atoms
* https://github.com/plutov/packagemain/blob/master/jwtdemo/main.go
