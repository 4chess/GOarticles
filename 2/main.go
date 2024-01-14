package main

import (
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
    articles         = make(map[int]string) // Map to store article titles
)

func main() {
    http.HandleFunc("/", serveForm)
    http.HandleFunc("/upload", handleFormSubmission)
    http.Handle("/articles/", http.StripPrefix("/articles/", http.FileServer(http.Dir(articlePath))))

    // Start the server
    log.Println("Starting server on :7070")
    log.Fatal(http.ListenAndServe(":7070", nil))
}

func serveForm(w http.ResponseWriter, r *http.Request) {
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

func handleFormSubmission(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    err := r.ParseMultipartForm(32 << 20) // 32MB max memory
    if err != nil {
        http.Error(w, "Failed to parse form", http.StatusBadRequest)
        return
    }

    title := r.FormValue("title")
    message := r.FormValue("message")

    // Create directory for the article
    currentID := int(atomic.AddInt32(&articleID, 1)) - 1
    dirPath := filepath.Join(articlePath, strconv.Itoa(currentID))
    err = os.MkdirAll(dirPath, 0755)
    if err != nil {
        http.Error(w, "Failed to create directory", http.StatusInternalServerError)
        return
    }

    file, header, err := r.FormFile("file")
    fileName := ""
    if err != nil && err != http.ErrMissingFile {
        http.Error(w, "Failed to get uploaded file", http.StatusInternalServerError)
        return
    }
    if file != nil {
        defer file.Close()
        saveUploadedFile(file, dirPath)
        fileName = header.Filename
    }

    // Create and save the article HTML file
    saveArticlePage(title, message, dirPath, fileName)

    // Store the title for the main page
    articles[currentID] = title

    // Redirect to the new article
    http.Redirect(w, r, fmt.Sprintf("/articles/%d", currentID), http.StatusSeeOther)
}

func saveUploadedFile(file multipart.File, dirPath string) {
    dst, err := os.Create(filepath.Join(dirPath, "upload"))
    if err != nil {
        log.Printf("Failed to create file: %v\n", err)
        return
    }
    defer dst.Close()

    _, err = io.Copy(dst, file)
    if err != nil {
        log.Printf("Failed to save file: %v\n", err)
    }
}

func saveArticlePage(title, message, dirPath, fileName string) {
    backLink := `<a href="/">Back to Main Page</a>`
    fileHTML := ""

    if fileName != "" {
        ext := filepath.Ext(fileName)
        switch ext {
        case ".jpg", ".jpeg", ".png", ".gif":
            fileHTML = fmt.Sprintf(`<img src="upload" style="width: 200px; height: 200px; float: left; margin-right: 20px;" alt="Uploaded Image">`)
        case ".mp4":
            fileHTML = fmt.Sprintf(`<video width="200" height="200" controls style="float: left; margin-right: 20px;"><source src="upload" type="video/mp4">Your browser does not support the video tag.</video>`)
        case ".mp3":
            fileHTML = fmt.Sprintf(`<audio controls style="float: left; margin-right: 20px;"><source src="upload" type="audio/mpeg">Your browser does not support the audio element.</audio>`)
}
}
content := fmt.Sprintf(`%s<h1>%s</h1>%s<div style="clear: both;">%s</div>%s`, backLink, title, fileHTML, message, backLink)
filePath := filepath.Join(dirPath, "index.html")
err := os.WriteFile(filePath, []byte(content), 0644)
if err != nil {
    log.Printf("Failed to save article: %v\n", err)
}
}