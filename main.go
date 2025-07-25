package main

import (
	"context"
	"database/sql"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/schema"
)

const name = "url-shortener"

const version = "0.0.9"

var revision = "HEAD"

//go:embed assets
var assets embed.FS

type ShortURL struct {
	bun.BaseModel `bun:"table:urls,alias:u"`

	ID        string    `bun:"id,pk" json:"id"`
	Original  string    `bun:"original,notnull" json:"original"`
	CreatedAt time.Time `bun:"created_at,notnull" json:"created_at"`
}

type ShortURLRequest struct {
	URL string `json:"url" validate:"required,url"`
}

type App struct {
	db *bun.DB
}

type Response struct {
	ShortURL string `json:"short_url,omitempty"`
	Error    string `json:"error,omitempty"`
}

func initDB(db *bun.DB) error {
	_, err := db.NewCreateTable().
		Model((*ShortURL)(nil)).
		IfNotExists().
		Exec(context.Background())
	return err
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateShortID(length int) string {
	var sb strings.Builder
	for i := 0; i < length; i++ {
		sb.WriteByte(charset[rand.Intn(len(charset))])
	}
	return sb.String()
}

func (app *App) shortenURLHandler(c echo.Context) error {
	var req ShortURLRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: "Invalid request body"})
	}

	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		return c.JSON(http.StatusBadRequest, Response{Error: "Invalid URL"})
	}

	ctx := c.Request().Context()
	var shortID string
	for {
		shortID = generateShortID(6)
		var exists ShortURL
		err := app.db.NewSelect().Model(&exists).Where("id = ?", shortID).Scan(ctx)
		if err != nil && err != sql.ErrNoRows {
			return c.JSON(http.StatusInternalServerError, Response{Error: "Database error"})
		}
		if err == sql.ErrNoRows {
			break
		}
	}

	shortURL := &ShortURL{
		ID:        shortID,
		Original:  req.URL,
		CreatedAt: time.Now(),
	}
	_, err := app.db.NewInsert().Model(shortURL).Exec(ctx)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, Response{Error: "Failed to save URL"})
	}

	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://"
		if c.Request().TLS != nil {
			serverURL = "https://"
		}
		serverURL += c.Request().Host
	}
	log.Println("server", serverURL)
	u, err := url.Parse(serverURL)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, Response{Error: "Failed to save URL"})
	}
	u.Path = shortID
	return c.JSON(http.StatusOK, Response{ShortURL: u.String()})
}

func (app *App) redirectHandler(c echo.Context) error {
	shortID := c.Param("id")
	ctx := c.Request().Context()

	var url ShortURL
	err := app.db.NewSelect().Model(&url).Where("id = ?", shortID).Scan(ctx)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, Response{Error: "Short URL not found"})
	} else if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Error: "Database error"})
	}

	return c.Redirect(http.StatusMovedPermanently, url.Original)
}

func main() {
	var ver bool
	flag.BoolVar(&ver, "version", false, "show version")
	flag.Parse()

	if ver {
		fmt.Println(version)
		os.Exit(0)
	}

	dsn := os.Getenv("DATABASE_URL")
	driver := sqliteshim.ShimName
	var dialect schema.Dialect = sqlitedialect.New()
	if dsn == "" {
		dsn = "file:./url-shortener.sqlite"
	}

	if strings.HasPrefix(dsn, "postgres://") {
		driver = "postgres"
		dialect = pgdialect.New()
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		log.Fatal(err)
	}

	bundb := bun.NewDB(db, dialect)
	defer bundb.Close()

	_, err = bundb.NewCreateTable().Model((*ShortURL)(nil)).IfNotExists().Exec(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	app := &App{db: bundb}

	e := echo.New()
	e.POST("/shorten", app.shortenURLHandler)
	e.GET("/:id", app.redirectHandler)

	sub, _ := fs.Sub(assets, "assets")
	e.GET("/", echo.WrapHandler(http.FileServer(http.FS(sub))))
	e.GET("/static/*", echo.WrapHandler(http.FileServer(http.FS(sub))))

	log.Println("Server starting on :8080")
	if err := e.Start(":8080"); err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}
