package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

func main() {

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "cloud run invoker")
	})

	e.GET("/favicon.ico", func(c echo.Context) error {
		return c.String(http.StatusOK, "")
	})

	// /* is the path to the cloud run service
	e.Any("/*", func(c echo.Context) error {
		fullPath := c.Request().URL.Path
		query := c.Request().URL.RawQuery

		aud := audience(c)

		audWithoutScheme := strings.Replace(aud, "https://", "", 1)
		fullPathWithoutAudience := strings.Replace(fullPath, "/"+audWithoutScheme, "", 1)
		fullPathWithoutAudience = strings.Replace(fullPathWithoutAudience, aud, "", 1)
		url := fmt.Sprintf("%s%s", aud, fullPathWithoutAudience)
		if query != "" {
			url = fmt.Sprintf("%s?%s", url, query)
		}

		logger.Info(url)

		err := proxyRequest(c, aud, url, c.Request().Method)
		if err != nil {
			logger.Error("error", slog.String("message", err.Error()))
			return c.String(http.StatusInternalServerError, err.Error())
		}

		return nil
	})

	e.Logger.Fatal(e.Start(":" + port))
}

const serviceName = "cloud-run-invoker"

func audience(c echo.Context) string {
	// if has cookie, return cookie value
	cookies := c.Cookies()
	for _, cookie := range cookies {
		if cookie.Name == serviceName {
			return cookie.Value
		}
	}

	// if not, generate audience from path
	fullPath := c.Request().URL.Path
	aud := fmt.Sprintf("https://%s", strings.Split(fullPath, "/")[1])
	c.SetCookie(&http.Cookie{
		Name:     serviceName,
		Value:    aud,
		Secure:   true,
		HttpOnly: true,
	})
	return aud
}

func proxyRequest(c echo.Context, audience, url, method string) error {

	ctx := context.Background()

	tokenSource, err := findToken(ctx, audience)
	if err != nil {
		return err
	}

	token, err := tokenSource.Token()
	if err != nil {
		return err
	}

	client := &http.Client{
		Timeout: time.Duration(10 * time.Second),
	}

	req, err := http.NewRequest(method, url, c.Request().Body)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	c.Response().Header().Set(echo.HeaderContentType, resp.Header.Get("Content-Type"))
	c.Response().WriteHeader(resp.StatusCode)
	io.Copy(c.Response(), resp.Body)

	return nil
}
