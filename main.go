package main

import (
    "encoding/json"
    "fmt"
    "html/template"
    "io"
    "log"
    "mime/multipart"
    "net/http"
    "os"
    "path/filepath"
    "strconv"
    "sync/atomic"
)

var (
    articleID   int32 = 1
    articlePath      = "./articles"
)

type Article struct {
    ID    int    `json:"id"`
    Title string `json:"title"`
}

type ArticlePageData struct {
    Title   string
    Content string
    Image   string
    Video   string
    Audio   string
}

func main() {
    articles, err := loadArticles()
    if err != nil {
        log.Fatalf("Failed to load articles: %v", err)
    }

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        serveForm(w, r, articles)
    })
    http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
        handleFormSubmission(w, r, &articles)
    })
    http.Handle("/articles/", http.StripPrefix("/articles/", http.FileServer(http.Dir(articlePath))))
    http.Handle("/style.css", http.FileServer(http.Dir("./")))

    log.Println("Starting server on :7070")
    log.Fatal(http.ListenAndServe(":7070", nil))
}

// loadArticles loads articles from the JSON file.
func loadArticles() ([]Article, error) {
    var articles []Article
    file, err := os.ReadFile("articles.json")
    if err != nil {
        if os.IsNotExist(err) {
            return articles, nil
        }
        return nil, err
    }
    err = json.Unmarshal(file, &articles)
    return articles, err
}

// saveArticles saves articles to the JSON file.
func saveArticles(articles []Article) error {
    file, err := json.Marshal(articles)
    if err != nil {
        return err
    }
    return os.WriteFile("articles.json", file, 0644)
}

// serveForm serves the form for submitting articles.
func serveForm(w http.ResponseWriter, r *http.Request, articles []Article) {
    if r.Method != "GET" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    t, err := template.ParseFiles("form.html")
    if err != nil {
        log.Printf("Error parsing HTML file: %v\n", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }
    t.Execute(w, articles)
}

// handleFormSubmission handles the submission of new articles.
func handleFormSubmission(w http.ResponseWriter, r *http.Request, articles *[]Article) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    err := r.ParseMultipartForm(30 << 20) // 30MB max memory
    if err != nil {
        http.Error(w, "Failed to parse form", http.StatusBadRequest)
        return
    }

    title := r.FormValue("title")
    if len(title) == 0 || len(title) > 75 {
        http.Error(w, "Title length must be between 1 and 75 characters", http.StatusBadRequest)
        return
    }

    message := r.FormValue("message")
    if len(message) == 0 || len(message) > 999999 {
        http.Error(w, "Article content too long", http.StatusBadRequest)
        return
    }

    currentID := int(atomic.AddInt32(&articleID, 1)) - 1
    dirPath := filepath.Join(articlePath, strconv.Itoa(currentID))
    err = os.MkdirAll(dirPath, 0755)
   
if err != nil {
    log.Printf("Failed to create directory: %v\n", err)
    http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    return
}

file, header, err := r.FormFile("file")
if err != nil && err != http.ErrMissingFile {
    log.Printf("Failed to get uploaded file: %v\n", err)
    http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    return
}

fileName := ""
if file != nil {
    defer file.Close()
    if header.Size > 30<<20 { // 30MB max file size
        http.Error(w, "File too large. Max size is 30MB", http.StatusBadRequest)
        return
    }
    fileName = header.Filename
    if !saveUploadedFile(file, dirPath) {
        http.Error(w, "Failed to save file", http.StatusInternalServerError)
        return
    }
}

if !saveArticlePage(title, message, dirPath, fileName) {
    http.Error(w, "Failed to save article", http.StatusInternalServerError)
    return
}

*articles = append([]Article{{ID: currentID, Title: title}}, *articles...)
if err := saveArticles(*articles); err != nil {
    log.Printf("Failed to save articles: %v\n", err)
    http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    return
}

http.Redirect(w, r, fmt.Sprintf("/articles/%d", currentID), http.StatusSeeOther)
}
// saveUploadedFile saves the uploaded file to the server.
func saveUploadedFile(file multipart.File, dirPath string) bool {
dst, err := os.Create(filepath.Join(dirPath, "upload"))
if err != nil {
log.Printf("Failed to create file: %v\n", err)
return false
}
defer dst.Close()
_, err = io.Copy(dst, file)
if err != nil {
    log.Printf("Failed to save file: %v\n", err)
    return false
}
return true
}

// saveArticlePage saves the article page to the server.
func saveArticlePage(title, message, dirPath, fileName string) bool {
data := ArticlePageData{
Title: title,
Content: message,
}
if fileName != "" {
    ext := filepath.Ext(fileName)
    switch ext {
    case ".jpg", ".jpeg", ".png", ".gif":
        data.Image = "upload"
    case ".mp4":
        data.Video = "upload"
    case ".mp3":
        data.Audio = "upload"
    }
}

tmpl, err := template.ParseFiles("articles.html")
if err != nil {
    log.Printf("Failed to parse article template: %v\n", err)
    return false
}

filePath := filepath.Join(dirPath, "index.html")
file, err := os.Create(filePath)
if err != nil {
    log.Printf("Failed to create article file: %v\n", err)
    return false
}
defer file.Close()

err = tmpl.Execute(file, data)
if err != nil {
    log.Printf("Failed to execute template: %v\n", err)
    return false
}
return true
}