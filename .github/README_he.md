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
  <!-- <a href="https://github.com/gofiber/fiber/blob/master/.github/README_he.md">
    <img height="20px" src="https://github.com/gofiber/docs/blob/master/static/flags/il.svg">
  </a> -->
  <br><br>
  <div dir="rtl">
  <a href="https://github.com/gofiber/fiber/releases">
    <img src="https://img.shields.io/github/release/gofiber/fiber?style=flat-square">
  </a>
  <a href="https://pkg.go.dev/github.com/gofiber/fiber?tab=doc">
    <img src="https://img.shields.io/badge/go.dev-007d9c?logo=go&logoColor=white&style=flat-square">
  </a>
  <a href="https://docs.gofiber.io">
    <img src="https://img.shields.io/badge/api-docs-blue?style=flat-square">
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
  <a href="https://t.me/gofiber">
    <img src="https://img.shields.io/badge/telegram-join%20chat-0088cc?style=flat-square">
  </a>
  </div>
</p>
<p align="center">
  <div dir="rtl">

  <b>Fiber</b> היא <b>web framework</b> בהשראת <a href="https://github.com/expressjs/express">Express</a> הבנויה על גבי <a href="https://github.com/valyala/fasthttp">Fasthttp</a>, מנוע ה-HTTP <b>המהיר ביותר</b> עבור <a href="https://golang.org/doc/">Go</a>.  
  נועדה <b>להקל</b> על העניינים למען פיתוח <b>מהיר</b>, <b>ללא הקצאות זכרון</b> ולוקחת <b>ביצועים</b> בחשבון.  
  </div>
</p>

<div dir="rtl">

## ⚡️ התחלה מהירה
</div>

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

<div dir="rtl">

## ⚙️ התקנה
</div>

<div dir="rtl">

קודם כל, [הורידו](https://golang.org/dl/) והתקינו את Go. נדרשת גרסה <span dir="ltr">`1.11`</span> ומעלה.
</div>

<div dir="rtl">

ההתקנה מתבצעת באמצעות הפקודה <span dir="ltr">[`go get`](https://golang.org/cmd/go/#hdr-Add_dependencies_to_current_module_and_install_them)</span>:
</div>

```bash
go get -u github.com/gofiber/fiber
```

<div dir="rtl">

## 🤖 מדדים
</div>

<div dir="rtl">

הבדיקות מבוצעות על ידי [TechEmpower](https://github.com/TechEmpower/FrameworkBenchmarks) ו-[Go Web](https://github.com/smallnest/go-web-framework-benchmark). אם אתם רוצים לראות את כל התוצאות, אנא בקרו ב-[Wiki](https://docs.gofiber.io/benchmarks) שלנו.
</div>

<p float="left" align="middle">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark-pipeline.png" width="49%">
  <img src="https://github.com/gofiber/docs/blob/master/.gitbook/assets//benchmark_alloc.png" width="49%">
</p>

<div dir="rtl">

## 🎯 יכולות
</div>

<div dir="rtl">

- [ניתוב](https://docs.gofiber.io/routing) רובסטי
- הנגשת [קבצים סטטיים](https://docs.gofiber.io/application#static)
- [ביצועים](https://docs.gofiber.io/benchmarks) גבוהים במיוחד
- צורך כמות [זכרון קטנה](https://docs.gofiber.io/benchmarks)
- [נקודות קצה עבור API](https://docs.gofiber.io/context)
- תמיכה ב-[Middleware](https://docs.gofiber.io/middleware) & [Next](https://docs.gofiber.io/context#next)
- תכנות [מהיר](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) של צד שרת
- [מנועי תבניות](https://docs.gofiber.io/middleware#template)
- [תמיכה ב-WebSocket](https://docs.gofiber.io/middleware#websocket)
- [הגבלת קצבים ובקשות](https://docs.gofiber.io/middleware#limiter)
- תורגם ל-12 שפות אחרות
- והרבה יותר, [חקור את Fiber](https://docs.gofiber.io/)
</div>

<div dir="rtl">

## 💡 פילוסופיה
</div>

<div dir="rtl">

gophers חדשים שעושים את המעבר מ-[Node.js](https://nodejs.org/en/about/) ל-[Go](https://golang.org/doc/) מתמודדים עם עקומת למידה לפני שהם יכולים להתחיל לבנות את יישומי האינטרנט או המיקרו-שירותים שלהם.  
Fiber כ-**web framework**, נוצרה עם רעיון **המינימליזם** ועוקבת אחרי **הדרך של UNIX**, כך ש-gophers חדשים יוכלו להיכנס במהירות לעולם של Go עם קבלת פנים חמה ואמינה.
</div>

<div dir="rtl">

Fiber נוצרה **בהשראת** Express, ה-web framework הפופולרית ביותר ברחבי האינטרנט. שילבנו את **הקלות** של Express ו**הביצועים הגולמיים** של Go. אם אי-פעם מימשתם יישום web ב-Node.js (_באמצעות Express או דומיו_), אז הרבה מהפונקציות והעקרונות ייראו לכם **מאוד מוכרים**.
</div>

<div dir="rtl">

אנחנו **מקשיבים** למשתמשים שלנו ב-[issues](https://github.com/gofiber/fiber/issues) (_ובכל רחבי האינטרנט_) כדי ליצור web framework **מהירה**, **גמישה**, ו**ידידותית** בשפת Go עבור **כל** משימה, **תאריך יעד** ו**כישורי** מפתח! בדיוק כמו ש-Express מבצע בעולם של JavaScript.
</div>

<div dir="rtl">

## 👀 דוגמאות
</div>

<div dir="rtl">

להלן כמה מהדוגמאות הנפוצות.
</div>

<div dir="rtl">

> אם ברצונכם לראות דוגמאות קוד נוספות, אנא בקרו ב[מאגר המתכונים](https://github.com/gofiber/recipes) שלנו או בקרו ב[תיעוד ה-API](https://docs.gofiber.io) שלנו.
</div>


<div dir="rtl">

### ניתוב
</div>

<div dir="rtl">

📖 [ניתוב](https://docs.gofiber.io/#basic-routing)  
</div>

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

<div dir="rtl">

### הנגשת קבצים סטטיים
</div>

<div dir="rtl">

📖 [קבצים סטטיים](https://docs.gofiber.io/application#static)  
</div>

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

<div dir="rtl">

### Middleware & Next
</div>

<div dir="rtl">

📖 [Middleware](https://docs.gofiber.io/routing#middleware)  
📖 [Next](https://docs.gofiber.io/context#next)  
</div>

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

<div dir="rtl">
<details>
  <summary>📚 הצג דוגמאות קוד נוספות</summary>
  

### מנועי תבניות

📖 [הגדרות](https://docs.gofiber.io/application#settings)  
📖 [רנדור](https://docs.gofiber.io/context#render)  
📖 [תבניות](https://docs.gofiber.io/middleware#template)  

Fiber תומך כברירת מחדל ב[מנוע התבניות של Go](https://golang.org/pkg/html/template/).

אבל אם ברצונכם להשתמש במנוע תבניות אחר כמו [amber](https://github.com/eknkc/amber), [handlebars](https://github.com/aymerick/raymond), [mustache](https://github.com/cbroglie/mustache) או [pug](https://github.com/Joker/jade).

אתם יכולים להשתמש ב[Middleware של התבניות](https://docs.gofiber.io/middleware#template) שלנו.

<div dir="ltr">

```go
package main

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

### קיבוץ routes ל-chains

📖 [קבוצות](https://docs.gofiber.io/application#group)  

<div dir="ltr">

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

### Middleware של לוגים

📖 [Logger](https://docs.gofiber.io/middleware#logger)  

<div dir="ltr">

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

### שיתוף משאבים בין מקורות (CORS)

📖 [CORS](https://docs.gofiber.io/middleware#cors)  

<div dir="ltr">

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

בדוק את ה-CORS על ידי העברת כל domain ב-header של <span dir="ltr">`Origin`</span>:

<div dir="ltr">

```bash
curl -H "Origin: http://example.com" --verbose http://localhost:3000
```
</div>

### תגובת 404 מותאמת אישית

📖 [שיטות HTTP](https://docs.gofiber.io/application#http-methods)  

<div dir="ltr">

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

### תגובת JSON

📖 [JSON](https://docs.gofiber.io/context#json)  

<div dir="ltr">

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

<div dir="ltr">

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

### Middleware של התאוששות

📖 [התאוששות](https://docs.gofiber.io/middleware#recover)  

<div dir="ltr">

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
</div>

<div dir="rtl">

## 🧬 Middlewares זמינים
</div>

<div dir="rtl">

למען עבודה _קלה וברורה יותר_, שמנו את ה-[middleware](https://docs.gofiber.io/middleware) תחת repositories נפרדים:
</div>

<div dir="rtl">

- [Basic Authentication](https://github.com/gofiber/basicauth)
- [Key Authentication](https://github.com/gofiber/keyauth)
- [Compression](https://github.com/gofiber/compression)
- [Request ID](https://github.com/gofiber/requestid)
- [WebSocket](https://github.com/gofiber/websocket)
- [Rewrite](https://github.com/gofiber/rewrite)
- [Recover](https://github.com/gofiber/recover)
- [Limiter](https://github.com/gofiber/limiter)
- [Session](https://github.com/gofiber/session)
- [Logger](https://github.com/gofiber/logger)
- [Helmet](https://github.com/gofiber/helmet)
- [Embed](https://github.com/gofiber/embed)
- [PPROF](https://github.com/gofiber/pprof)
- [CORS](https://github.com/gofiber/cors)
- [CSRF](https://github.com/gofiber/csrf)
- [JWT](https://github.com/gofiber/jwt)
</div>

<div dir="rtl">

## 💬 מדיה
</div>

<div dir="ltr">

- [Welcome to Fiber — an Express.js styled web framework written in Go with ❤️](https://dev.to/koddr/welcome-to-fiber-an-express-js-styled-fastest-web-framework-written-with-on-golang-497) — _03 Feb 2020_
- [Fiber released v1.7! 🎉 What's new and is it still fast, flexible and friendly?](https://dev.to/koddr/fiber-v2-is-out-now-what-s-new-and-is-he-still-fast-flexible-and-friendly-3ipf) — _21 Feb 2020_
- [🚀 Fiber v1.8. What's new, updated and re-thinked?](https://dev.to/koddr/fiber-v1-8-what-s-new-updated-and-re-thinked-339h) — _03 Mar 2020_
- [Is switching from Express to Fiber worth it? 🤔](https://dev.to/koddr/are-sure-what-your-lovely-web-framework-running-so-fast-2jl1) — _01 Apr 2020_
- [Creating Fast APIs In Go Using Fiber](https://dev.to/jozsefsallai/creating-fast-apis-in-go-using-fiber-59m9) — _07 Apr 2020_
- [Building a Basic REST API in Go using Fiber](https://tutorialedge.net/golang/basic-rest-api-go-fiber/) - _23 Apr 2020_
</div>

<div dir="rtl">

## 👍 לתרום
</div>

<div dir="rtl">

אם אתם רוצים לומר **תודה** או/ו לתמוך בפיתוח הפעיל של <span dir="ltr">`Fiber`</span>:

</div>

<div dir="rtl">

1. תוסיפו [GitHub Star](https://github.com/gofiber/fiber/stargazers) לפרויקט.
2. צייצו לגבי הפרויקט [בטוויטר שלכם](https://twitter.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. כתבו ביקורת או מדריך ב-[Medium](https://medium.com/), [Dev.to](https://dev.to/) או בבלוג האישי שלכם.
4. עזרו לנו לתרגם את ה-<span dir="ltr">`README`</span> הזה לשפה אחרת.
5. תמכו בפרויקט על ידי תרומת [כוס קפה](https://buymeacoff.ee/fenny).
</div>


<div dir="rtl">

## ☕ תומכים
</div>

<div dir="rtl">

Fiber היא פרויקט קוד פתוח שתשלום חשובונתיו מסתמך על תרומות, כגון שם ה-domain שלנו, gitbook, netlify ו-serverless hosting. אם אתם רוצים לתמוך ב-Fiber, אתם יכולים ☕ [**קנו קפה כאן**](https://buymeacoff.ee/fenny)
</div>

|                                                             | משתמש                                           | תרומה |
| :---------------------------------------------------------- | :---------------------------------------------- | :---- |
| ![](https://avatars.githubusercontent.com/u/59947262?s=25 ) | [@thomasvvugt](https://github.com/thomasvvugt)  | ☕ x 5 |
| ![](https://avatars.githubusercontent.com/u/1094221?s=25 )  | [@ekaputra07](https://github.com/ekaputra07)    | ☕ x 5 |
| ![](https://avatars.githubusercontent.com/u/635852?s=25 )   | [@bihe](https://github.com/bihe)                | ☕ x 3 |
| ![](https://avatars.githubusercontent.com/u/59947262?s=25 ) | @justdave                                       | ☕ x 3 |
| ![](https://avatars.githubusercontent.com/u/11155743?s=25 ) | [@koddr](https://github.com/koddr)              | ☕ x 1 |
| ![](https://avatars.githubusercontent.com/u/5638101?s=25 )  | [@raymayemir](https://github.com/raymayemir)    | ☕ x 1 |
| ![](https://avatars.githubusercontent.com/u/619996?s=25 )   | [@melkorm](https://github.com/melkorm)          | ☕ x 1 |
| ![](https://avatars.githubusercontent.com/u/31022056?s=25 ) | [@marvinjwendt](https://github.com/thomasvvugt) | ☕ x 1 |
| ![](https://avatars.githubusercontent.com/u/31921460?s=25 ) | [@toishy](https://github.com/toishy)            | ☕ x 1 |


<div dir="rtl">

## ‎‍💻 תורמי קוד
</div>

<img src="https://opencollective.com/fiber/contributors.svg?width=890&button=false" alt="Code Contributors" style="max-width:100%;">

<div dir="rtl">

## ⚠️ רישיון
</div>

<div dir="ltr">

Copyright (c) 2019-present [Fenny](https://github.com/fenny) and [Contributors](https://github.com/gofiber/fiber/graphs/contributors). `Fiber` is free and open-source software licensed under the [MIT License](https://github.com/gofiber/fiber/blob/master/LICENSE). Official logo was created by [Vic Shóstak](https://github.com/koddr) and distributed under [Creative Commons](https://creativecommons.org/licenses/by-sa/4.0/) license (CC BY-SA 4.0 International).
</div>

<div dir="rtl">

**רישיונות של ספריות צד שלישי**
- [FastHTTP](https://github.com/valyala/fasthttp/blob/master/LICENSE)
- [Schema](https://github.com/gorilla/schema/blob/master/LICENSE)
</div>