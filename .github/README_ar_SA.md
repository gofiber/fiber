<p align="center">
  <a href="https://gofiber.io">
    <img alt="Fiber" height="125" src="https://github.com/gofiber/docs/blob/master/static/fiber_v2_logo.svg">
  </a>
  <br>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/en.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ru.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ru.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_es.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/es.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ja.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/jp.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_pt.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/pt.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_zh-CN.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ch.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_de.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/de.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_nl.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/nl.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ko.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/ko.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_fr.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/fr.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_tr.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/tr.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_id.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/id.svg">
  </a>
  <a href="https://github.com/gofiber/fiber/blob/master/.github/README_he.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/il.svg">
  </a>
<!--   <a href="https://github.com/gofiber/fiber/blob/master/.github/README_ar_SA.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/sa.svg">
  </a> -->
  <br><br>
  <a href="https://github.com/gofiber/fiber/releases">
    <img src="https://img.shields.io/github/release/gofiber/fiber?style=flat-square">
  </a>
  <a href="https://docs.gofiber.io">
    <img src="https://img.shields.io/badge/api-docs-blue?style=flat-square">
  </a>
  <a href="https://pkg.go.dev/github.com/gofiber/fiber?tab=doc">
    <img src="https://img.shields.io/badge/go.dev-007d9c?logo=go&logoColor=white&style=flat-square">
  </a>
  <a href="https://goreportcard.com/report/github.com/gofiber/fiber">
    <img src="https://goreportcard.com/badge/github.com/gofiber/fiber?style=flat-square">
  </a>
  <a href="https://gocover.io/github.com/gofiber/fiber">
    <img src="https://img.shields.io/badge/coverage-91%25-brightgreen?style=flat-square">
  </a>
  <a href="https://github.com/gofiber/fiber/actions?query=workflow%3ATest">
    <img src="https://img.shields.io/github/workflow/status/gofiber/fiber/Test?label=tests&style=flat-square">
  </a>
  <a href="https://github.com/gofiber/fiber/actions?query=workflow%3AGosec">
    <img src="https://img.shields.io/github/workflow/status/gofiber/fiber/Gosec?label=gosec&style=flat-square">
  </a>
  <a href="https://gofiber.io/discord">
    <img src="https://img.shields.io/badge/discord-join%20channel-7289DA?style=flat-square">
  </a>
</p>

<p align="center">
 <div dir="rtl">
  <b>Fiber</b> هو <b>إطار ويب</b>  مستوحى من <a href="https://github.com/expressjs/express">Express</a>   مبني على <a href="https://github.com/valyala/fasthttp">Fasthttp</a>,  <b>اسرع</b> محرك HTTP  لـ <a href="https://golang.org/doc/">Go</a>. مصمم ليكون <b>سهل</b> لأغراض <b>السرعة</b> مع عدم  <b>تخصيص ذاكرة والأداء</b> و <b>الاداء العالي</b> دائما.
 <div dir="rtl">
</p>

## ⚡️ بداية سريعة

<div dir="ltr">


```go
package main

import "github.com/gofiber/fiber"

func main() {
  app := fiber.New()

  app.Get("/", func(c *fiber.Ctx) {
    c.Send("Hello, World!")
  })

  app.Listen(3000)
}
```


</div>

## ⚙️ تثبيت 

قبل كل شي قم , [بتحميل](https://golang.org/dl/)   و تثبيت  Go. `1.11` أو أعلى مطلوب.

بعد الانتهاء من التثبيت استخدم الامر [`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them) :

<div dir="ltr">


```bash
go get -u github.com/gofiber/fiber
```

</div>

## 🤖 مقايس الاداء

يتم تنفيذ هذه الاختبارات من قبل [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks) و [Go Web](https://github.com/smallnest/go-web-framework-benchmark). إذا كنت تريد رؤية جميع النتائج ، يرجى زيارة موقعنا [Wiki](https://docs.gofiber.io/benchmarks).

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark_alloc.png" width="49%">
</p>

## 🎯 الميزات

- قوي [routing](https://docs.gofiber.io/routing)
- يقدم خدمة [static files](https://docs.gofiber.io/application#static)
- أقصى [أداء](https://docs.gofiber.io/benchmarks)
- [ذاكرة منخفضة](https://docs.gofiber.io/benchmarks) 
- [API endpoints](https://docs.gofiber.io/context)
- [Middleware](https://docs.gofiber.io/middleware) & [Next](https://docs.gofiber.io/context#next) مدعوم
- [سريع](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) server-side programming
- [Template engines](https://docs.gofiber.io/middleware#template)
- [WebSocket دعم](https://docs.gofiber.io/middleware#websocket)
- [Rate Limiter](https://docs.gofiber.io/middleware#limiter)
- ترجم الى [12 لغة أخرى](https://docs.gofiber.io/)
- وأكثر بكثير, [استكشف Fiber](https://docs.gofiber.io/)

## 💡 فلسفة

قوفر(مستخدمي لغة Go  الجدد) جديد يجعل التبديل من [Node.js](https://nodejs.org/en/about/) الى [Go](https://golang.org/doc/)تتعامل مع منحنى التعلم قبل أن يتمكنوا من البدء في بناءتطبيقات الويب . Fiber, كـ **إطار الويب**, تم إنشاؤه بفكرة **minimalism** ويتبع **UNIX way**, حتى يتمكن القوفرون الجدد من دخول عالم Go بترحيب حار وموثوق.

Fiber هو **مستوحى** من Express, إطار الويب الأكثر شعبية على الإنترنت. قمنا بدمج **سهولة** الـ Express و **الأداء الخام** لـ Go. إذا كنت قد قمت بتطبيق تطبيق ويب في Node.js (_using Express or similar_), ستظهر العديد من الأساليب والمبادئ **الاكثر شيوعاً** لك.

نحن **نصغي** لمستخدمينا [issues](https://github.com/gofiber/fiber/issues), نناقش [channel](https://gofiber.io/discord) _وفي جميع أنحاء الإنترنت_ لإنشاء **سريع**, **مرن** و **مألوف** Go إطار الويب لـ **لأي** مهمة, **الموعد الأخير
** و تطوير **مهارات**! فقط مثل Express تفعل لـ JavaScript عالم.

## 👀 أمثلة

فيما يلي بعض الأمثلة الشائعة. 

> إذا كنت ترغب في رؤية المزيد من أمثلة التعليمات البرمجية, يرجى زيارة [Recipes repository](https://github.com/gofiber/recipes) او زيارة [API documentation](https://docs.gofiber.io).

### Routing

📖 [Routing](https://docs.gofiber.io/#basic-routing)  

<div dir="ltr" >


```go
func main() {
  app := fiber.New()

  // GET /john
  app.Get("/:name", func(c *fiber.Ctx) {
    fmt.Printf("Hello %s!", c.Params("name"))
    // => Hello john!
  })

  // GET /john
  app.Get("/:name/:age?", func(c *fiber.Ctx) {
    fmt.Printf("Name: %s, Age: %s", c.Params("name"), c.Params("age"))
    // => Name: john, Age:
  })

  // GET /api/register
  app.Get("/api/*", func(c *fiber.Ctx) {
    fmt.Printf("/api/%s", c.Params("*"))
    // => /api/register
  })

  app.Listen(3000)
}
```

</div>

### يخدم static files

📖 [Static](https://docs.gofiber.io/application#static)  

<div dir="ltr">

```go
func main() {
  app := fiber.New()

  app.Static("/", "/public")
  // => http://localhost:3000/js/script.js
  // => http://localhost:3000/css/style.css

  app.Static("/prefix", "/public")
  // => http://localhost:3000/prefix/js/script.js
  // => http://localhost:3000/prefix/css/style.css

  app.Static("*", "/public/index.html")
  // => http://localhost:3000/any/path/shows/index/html

  app.Listen(3000)
}
```
</div>

### Middleware & Next

📖 [Middleware](https://docs.gofiber.io/routing#middleware)  
📖 [Next](https://docs.gofiber.io/context#next)  

<div dir="ltr">

```go
func main() {
  app := fiber.New()

  // Match any route
  app.Use(func(c *fiber.Ctx) {
    fmt.Println("First middleware")
    c.Next()
  })

  // Match all routes starting with /api
  app.Use("/api", func(c *fiber.Ctx) {
    fmt.Println("Second middleware")
    c.Next()
  })

  // GET /api/register
  app.Get("/api/list", func(c *fiber.Ctx) {
    fmt.Println("Last middleware")
    c.Send("Hello, World!")
  })

  app.Listen(3000)
}
```

</div>

<details>
  <summary>📚 إظهار المزيد من أمثلة التعليمات البرمجية</summary>

### Template engines

📖 [Settings](https://docs.gofiber.io/application#settings)  
📖 [Render](https://docs.gofiber.io/context#render)  
📖 [Template](https://docs.gofiber.io/middleware#template)  

Fiber يدعم وبشكل افتراضي [Go template engine](https://golang.org/pkg/html/template/)

ولكن إذا كنت ترغب في استخدام محرك قالب آخر مثل [amber](https://github.com/eknkc/amber), [handlebars](https://github.com/aymerick/raymond), [mustache](https://github.com/cbroglie/mustache) او [pug](https://github.com/Joker/jade).

يمكنك استخدام  [Template Middleware](https://docs.gofiber.io/middleware#template).

<div dir="ltr" >

```go
import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/template"
)

func main() {
  // You can setup template engine before initiation app:
  app := fiber.New(&fiber.Settings{
    TemplateEngine:    template.Mustache(),
    TemplateFolder:    "./views",
    TemplateExtension: ".tmpl",
  })

  // OR after initiation app at any convenient location:
  app.Settings.TemplateEngine = template.Mustache()
  app.Settings.TemplateFolder = "./views"
  app.Settings.TemplateExtension = ".tmpl"

  // And now, you can call template `./views/home.tmpl` like this:
  app.Get("/", func(c *fiber.Ctx) {
    c.Render("home", fiber.Map{
      "title": "Homepage",
      "year":  1999,
    })
  })

  // ...
}
```

</div>

### Grouping routes into chains

📖 [Group](https://docs.gofiber.io/application#group)  

<div dir="ltr" >

```go
func main() {
  app := fiber.New()

  // Root API route
  api := app.Group("/api", cors())  // /api

  // API v1 routes
  v1 := api.Group("/v1", mysql())   // /api/v1
  v1.Get("/list", handler)          // /api/v1/list
  v1.Get("/user", handler)          // /api/v1/user

  // API v2 routes
  v2 := api.Group("/v2", mongodb()) // /api/v2
  v2.Get("/list", handler)          // /api/v2/list
  v2.Get("/user", handler)          // /api/v2/user

  // ...
}
```

</div>

### Middleware logger

📖 [Logger](https://docs.gofiber.io/middleware#logger)  

<div dir="ltr" >

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/logger"
)

func main() {
    app := fiber.New()

    // Optional logger config
    config := logger.Config{
      Format:     "${time} - ${method} ${path}\n",
      TimeFormat: "Mon, 2 Jan 2006 15:04:05 MST",
    }

    // Logger with config
    app.Use(logger.New(config))

    app.Listen(3000)
}
```

</div>

### Cross-Origin Resource Sharing (CORS)

📖 [CORS](https://docs.gofiber.io/middleware#cors)  

<div dir="ltr" >

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/cors"
)

func main() {
    app := fiber.New()

    // CORS with default config
    app.Use(cors.New())

    app.Listen(3000)
}
```

</div>

التحقق من CORS عن طريق تمرير أي مجال `Origin` العنوان:

<div dir="ltr" >

```bash
curl -H "Origin: http://example.com" --verbose http://localhost:3000
```
</div>


### مخصص 404 response

📖 [HTTP Methods](https://docs.gofiber.io/application#http-methods)  


<div dir="ltr" >

```go
func main() {
  app := fiber.New()

  app.Static("/public")

  app.Get("/demo", func(c *fiber.Ctx) {
    c.Send("This is a demo!")
  })

  app.Post("/register", func(c *fiber.Ctx) {
    c.Send("Welcome!")
  })

  // Last middleware to match anything
  app.Use(func(c *fiber.Ctx) {
    c.SendStatus(404) 
    // => 404 "Not Found"
  })

  app.Listen(3000)
}
```
</div>

### JSON Response

📖 [JSON](https://docs.gofiber.io/context#json)  

<div dir="ltr" >

```go
type User struct {
  Name string `json:"name"`
  Age  int    `json:"age"`
}

func main() {
  app := fiber.New()

  app.Get("/user", func(c *fiber.Ctx) {
    c.JSON(&User{"John", 20})
    // => {"name":"John", "age":20}
  })

  app.Get("/json", func(c *fiber.Ctx) {
    c.JSON(fiber.Map{
      "success": true,
      "message": "Hi John!",
    })
    // => {"success":true, "message":"Hi John!"}
  })

  app.Listen(3000)
}
```
</div>



### WebSocket Upgrade

📖 [Websocket](https://docs.gofiber.io/middleware#websocket)  

<div dir="ltr" >

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/websocket"
)

func main() {
  app := fiber.New()

  app.Get("/ws", websocket.New(func(c *websocket.Conn) {
    for {
      mt, msg, err := c.ReadMessage()
      if err != nil {
        log.Println("read:", err)
        break
      }
      log.Printf("recv: %s", msg)
      err = c.WriteMessage(mt, msg)
      if err != nil {
        log.Println("write:", err)
        break
      }
    }
  }))

  app.Listen(3000)
  // ws://localhost:3000/ws
}
```

</div>

### Recover middleware

📖 [Recover](https://docs.gofiber.io/middleware#recover)  

<div dir="ltr" >

```go
import (
    "github.com/gofiber/fiber"
    "github.com/gofiber/recover"
)

func main() {
  app := fiber.New()

  // Optional recover config
  config := recover.Config{
    Handler: func(c *fiber.Ctx, err error) {
			c.SendString(err.Error())
			c.SendStatus(500)
		},
  }

  // Logger with custom config
  app.Use(recover.New(config))

  app.Listen(3000)
}
```

</div>
</details>

## 🧬 الرسمية Middlewares

والمزيد _قابلة للصيانة_ middleware _ecosystem_, لقد وضعنا رسمية [middlewares](https://docs.gofiber.io/middleware) في مستودعات منفصلة:

- [gofiber/compression](https://github.com/gofiber/compression)
- [gofiber/basicauth](https://github.com/gofiber/basicauth)
- [gofiber/requestid](https://github.com/gofiber/requestid)
- [gofiber/websocket](https://github.com/gofiber/websocket)
- [gofiber/keyauth](https://github.com/gofiber/keyauth)
- [gofiber/rewrite](https://github.com/gofiber/rewrite)
- [gofiber/recover](https://github.com/gofiber/recover)
- [gofiber/limiter](https://github.com/gofiber/limiter)
- [gofiber/session](https://github.com/gofiber/session)
- [gofiber/adaptor](https://github.com/gofiber/adaptor)
- [gofiber/logger](https://github.com/gofiber/logger)
- [gofiber/helmet](https://github.com/gofiber/helmet)
- [gofiber/embed](https://github.com/gofiber/embed)
- [gofiber/pprof](https://github.com/gofiber/pprof)
- [gofiber/cors](https://github.com/gofiber/cors)
- [gofiber/csrf](https://github.com/gofiber/csrf)
- [gofiber/jwt](https://github.com/gofiber/jwt)

## 🌱 Third Party Middlewares

هذه قائمة middlewares التي تم إنشاؤها من قبل المجتمع Fiber , الرجاء إنشاءPR إذا كنت تريد أن ترى ذلك!
- [arsmn/fiber-swagger](https://github.com/arsmn/fiber-swagger)
- [arsmn/fiber-casbin](https://github.com/arsmn/fiber-casbin)
- [arsmn/fiber-introspect](https://github.com/arsmn/fiber-introspect)
- [shareed2k/fiber_tracing](https://github.com/shareed2k/fiber_tracing)
- [shareed2k/fiber_limiter](https://github.com/shareed2k/fiber_limiter)
- [thomasvvugt/fiber-boilerplate](https://github.com/thomasvvugt/fiber-boilerplate)
- [arsmn/gqlgen](https://github.com/arsmn/gqlgen)

## 💬 وسائل الإعلام

- [Welcome to Fiber — an Express.js styled web framework written in Go with ❤️](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) — _03 Feb 2020_
- [Fiber released v1.7! 🎉 What's new and is it still fast, flexible and friendly?](https://dev.to/koddr/fiber-v2-is-out-now-what-s-new-and-is-he-still-fast-flexible-and-friendly-3ipf) — _21 Feb 2020_
- [🚀 Fiber v1.8. What's new, updated and re-thinked?](https://dev.to/koddr/fiber-v1-8-what-s-new-updated-and-re-thinked-339h) — _03 Mar 2020_
- [Is switching from Express to Fiber worth it? 🤔](https://dev.to/koddr/are-sure-what-your-lovely-web-framework-running-so-fast-2jl1) — _01 Apr 2020_
- [Creating Fast APIs In Go Using Fiber](https://dev.to/jozsefsallai/creating-fast-apis-in-go-using-fiber-59m9) — _07 Apr 2020_
- [Building a Basic REST API in Go using Fiber](https://tutorialedge.net/golang/basic-rest-api-go-fiber/) - _23 Apr 2020_
- [📺 Building a REST API using GORM and Fiber](https://youtu.be/Iq2qT0fRhAA) - _25 Apr 2020_
- [🌎 Create a travel list app with Go, Fiber, Angular, MongoDB and Google Cloud Secret Manager](https://blog.yongweilun.me/create-a-travel-list-app-with-go-fiber-angular-mongodb-and-google-cloud-secret-manager-ck9fgxy0p061pcss1xt1ubu8t) - _25 Apr 2020_
- [Fiber v1.9.6 🔥 How to improve performance by 817% and stay fast, flexible and friendly?](https://dev.to/koddr/fiber-v1-9-5-how-to-improve-performance-by-817-and-stay-fast-flexible-and-friendly-2dp6) - _12 May 2020_

## 👍 مساهمة

إذا كنت تريد أن تقول **شكرا جزيل** و/او دعم التنمية النشطة للـ `Fiber`:

1. اضف [GitHub نجمة](https://github.com/gofiber/fiber/stargazers) للمشروع.
2. غرد عن المشروع [في تويتر ](https://twitter.com/intent/tweet?text=Fiber%20is%20an%20Express%20inspired%20%23web%20%23framework%20built%20on%20top%20of%20Fasthttp%2C%20the%20fastest%20HTTP%20engine%20for%20%23Go.%20Designed%20to%20ease%20things%20up%20for%20%23fast%20development%20with%20zero%20memory%20allocation%20and%20%23performance%20in%20mind%20%F0%9F%9A%80%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. اكتب مراجعة أو برنامج تعليمي عن [Medium](https://medium.com/), [Dev.to](https://dev.to/) او في موقعك الشخصي.
4. ساعدنا في ترجمة موقعنا API التوثيق عبر [Crowdin](https://crowdin.com/project/gofiber) [![Crowdin](https://badges.crowdin.net/gofiber/localized.svg)](https://crowdin.com/project/gofiber)
5. دعم المشروع بالتبرع بـ [كوب من القهوة](https://buymeacoff.ee/fenny).

## ☕ الداعمين

Fiber هو مشروع مفتوح المصدر يعمل على التبرعات لدفع الفواتير ، على سبيل المثال اسم النطاق الخاص بنا , gitbook, netlify and serverless الاستضافة. إذا كنت تريد دعم Fiber, تستطيع ☕ [**شراء كوب قهوة هنا**](https://buymeacoff.ee/fenny).

|                                                             | المستخدم                                            | التبرع |
| :---------------------------------------------------------- | :---------------------------------------------- | :------- |
| ![](https://avatars.githubusercontent.com/u/59947262?s=25 ) | [@thomasvvugt](https://github.com/thomasvvugt)  | ☕ x 5    |
| ![](https://avatars.githubusercontent.com/u/1094221?s=25 )  | [@ekaputra07](https://github.com/ekaputra07)    | ☕ x 5    |
| ![](https://avatars.githubusercontent.com/u/186637?s=25 )   | [@candidosales](https://github.com/candidosales)| ☕ x 5    |
| ![](https://avatars.githubusercontent.com/u/635852?s=25 )   | [@bihe](https://github.com/bihe)                | ☕ x 3    |
| ![](https://avatars.githubusercontent.com/u/307334?s=25 )   | [@justdave](https://github.com/justdave)        | ☕ x 3    |
| ![](https://avatars.githubusercontent.com/u/11155743?s=25 ) | [@koddr](https://github.com/koddr)              | ☕ x 1    |
| ![](https://avatars.githubusercontent.com/u/29042462?s=25 ) | [@lapolinar](https://github.com/lapolinar)      | ☕ x 1    |
| ![](https://avatars.githubusercontent.com/u/2978730?s=25 )  | [@diegowifi](https://github.com/diegowifi)      | ☕ x 1    |
| ![](https://avatars.githubusercontent.com/u/44171355?s=25 ) | [@ssimk0](https://github.com/ssimk0)            | ☕ x 1    |
| ![](https://avatars.githubusercontent.com/u/5638101?s=25 )  | [@raymayemir](https://github.com/raymayemir)    | ☕ x 1    |
| ![](https://avatars.githubusercontent.com/u/619996?s=25 )   | [@melkorm](https://github.com/melkorm)          | ☕ x 1    |
| ![](https://avatars.githubusercontent.com/u/31022056?s=25 ) | [@marvinjwendt](https://github.com/thomasvvugt) | ☕ x 1    |
| ![](https://avatars.githubusercontent.com/u/31921460?s=25 ) | [@toishy](https://github.com/toishy)            | ☕ x 1    |

## ‎‍💻 المساهمون في كتابة الكود

<img src="https://opencollective.com/fiber/contributors.svg?width=890&button=false" alt="Code Contributors" style="max-width:100%;">

## ⚠️ رخصة

Copyright (c) 2019-present [Fenny](https://github.com/fenny) and [Contributors](https://github.com/gofiber/fiber/graphs/contributors). `Fiber` هو برنامج مجاني ومفتوح المصدر مرخص بموجب [MIT License](https://github.com/gofiber/fiber/blob/master/LICENSE). تم إنشاء الشعار الرسمي من قبل [Vic Shóstak](https://github.com/koddr) ووزعت تحت [Creative Commons](https://creativecommons.org/licenses/by-sa/4.0/) رخصة (CC BY-SA 4.0 International).

**Third-party library licenses**
- [FastHTTP](https://github.com/valyala/fasthttp/blob/master/LICENSE)
- [Schema](https://github.com/gorilla/schema/blob/master/LICENSE)
- [bytebufferpool](https://github.com/valyala/bytebufferpool/blob/master/LICENSE)
