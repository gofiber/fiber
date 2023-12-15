---
id: docker
title: 🐳 Build docker image
description: Build a simple docker image that can be used in production.
sidebar_position: 8
---

After we have completed the development of our application, the moment comes when we want to upload it to the real world, most often we use Docker for this, wrapping our application in a container.

To do this we need to follow a few simple steps:

## Step 1 - Setup project

First create folder for our project run the following command in the console:

```bash title="~/"
mkdir example && cd example
```

Then we have to init go project:
```bash title="~/example"
go mod init example.org/demo
```

Add **Fiber** to our project:
```bash title="~/example"
go get github.com/gofiber/fiber/v2
```

## Step 2 - Hello world

To start we should create `main.go` file:
```bash title="~/example"
touch main.go
```

Next we need to add the following content to our file:

```go title="~/app/main.go"
package main

import "github.com/gofiber/fiber/v2"

func main() {
	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	app.Listen(":3000")
}
```

## Step 3 - create Dockerfile

Run the following command in the console:

```bash
touch Dockerfile
```

For the image we will use [Distroless](https://github.com/GoogleContainerTools/distroless) docker image, in the file we should add following content:

```bash title="~/example/Dockerfile"
FROM golang:1.21 as build

WORKDIR /app
COPY . .

RUN go mod download && go mod verify
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /server .

FROM gcr.io/distroless/static-debian12
COPY --from=build /server .

ENV SOME_VAR=foo

EXPOSE 3000
CMD ["./server"]
```

:::info
"Distroless" images contain only your application and its runtime dependencies. They do not contain package managers, shells or any other programs you would expect to find in a standard Linux distribution.
:::

:::tip
A docker image size is important, because this image is going to be uploaded / downloaded during the build / deploy. The more size of image is the more time you will spend on network transmittion.

That's why we divided the build process into two stages:

1. "base" - in the Dockerfile we start building from — golang:1.21-alpine. Alpine is the tiniest linux image we currently have. It is image size is about 3MB.

2. use Distroless just to put binary file there.
:::

## Step 4 - build an image

For this you should execute fllowing command in console:
```bash title"~/example"
docker build -t <your_docker_image_tag> .
```

## Step 5 - profit

Now you can run the docker image using the command:
```bash
docker run -p 3000:3000 <your_docker_image_tag>
```

And open in the browser:
```
http://localhost:3000/
```