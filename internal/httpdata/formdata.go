package httpdata

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"time"
)

// FormField represents a form field with name and value.
type FormField struct {
	Name  string
	Value string
}

// FormType represents different types of form submissions.
type FormType string

const (
	FormTypeLogin   FormType = "login"
	FormTypeComment FormType = "comment"
	FormTypeContact FormType = "contact"
	FormTypeSearch  FormType = "search"
	FormTypeUpload  FormType = "upload"
)

// FormFieldNames contains common form field names by category.
var FormFieldNames = map[string][]string{
	"identity": {"username", "email", "user", "login", "name"},
	"auth":     {"password", "pass", "pwd", "token"},
	"content":  {"message", "comment", "content", "body", "text", "description"},
	"meta":     {"subject", "title", "topic"},
	"search":   {"search", "query", "q", "keyword"},
	"file":     {"file", "upload", "image", "document", "attachment"},
}

// ContentTypes contains form content type options.
var ContentTypes = []string{
	"application/x-www-form-urlencoded",
	"multipart/form-data",
	"application/json",
	"text/plain",
}

// FormEndpoints contains common form submission paths.
var FormEndpoints = []string{
	"/login", "/signin", "/comment", "/post", "/submit",
	"/contact", "/message", "/upload", "/api/submit",
	"/wp-comments-post.php", "/wp-admin/admin-ajax.php",
	"/index.php", "/submit.php", "/process.php",
	"/api/v1/submit", "/api/v2/data", "/graphql",
	"/newsletter", "/subscribe", "/feedback",
	"/register", "/signup", "/checkout",
}

// FormReferers contains referrer URLs by form type.
var FormReferers = map[FormType][]string{
	FormTypeLogin:   {"https://www.example.com/login", "https://accounts.example.com/signin"},
	FormTypeComment: {"https://www.example.com/blog/post", "https://news.example.com/article"},
	FormTypeContact: {"https://www.example.com/contact", "https://support.example.com/ticket"},
	FormTypeSearch:  {"https://www.example.com/search", "https://www.google.com/search"},
	FormTypeUpload:  {"https://www.example.com/upload", "https://drive.example.com/new"},
}

// FormDataGenerator generates realistic form data for various attack types.
type FormDataGenerator struct {
	UseJSON      bool
	UseMultipart bool
	FieldCount   int
}

// NewFormDataGenerator creates a generator with default settings.
func NewFormDataGenerator() *FormDataGenerator {
	return &FormDataGenerator{
		UseJSON:      false,
		UseMultipart: false,
		FieldCount:   5,
	}
}

// DetectFormType infers form type from URL path.
func DetectFormType(path string) FormType {
	pathLower := strings.ToLower(path)

	typePatterns := map[FormType][]string{
		FormTypeLogin:   {"login", "signin", "auth"},
		FormTypeComment: {"comment", "post", "reply"},
		FormTypeContact: {"contact", "message", "support"},
		FormTypeSearch:  {"search", "query", "find"},
		FormTypeUpload:  {"upload", "file", "attach"},
	}

	for formType, patterns := range typePatterns {
		for _, pattern := range patterns {
			if strings.Contains(pathLower, pattern) {
				return formType
			}
		}
	}

	return FormTypeLogin
}

// GenerateFields creates random form fields for the specified form type.
func (g *FormDataGenerator) GenerateFields(formType FormType) []FormField {
	fields := make([]FormField, 0, g.FieldCount)

	switch formType {
	case FormTypeLogin:
		fields = append(fields,
			FormField{Name: "username", Value: g.generateUsername()},
			FormField{Name: "password", Value: g.generatePassword()},
		)
	case FormTypeComment:
		fields = append(fields,
			FormField{Name: "name", Value: g.generateUsername()},
			FormField{Name: "email", Value: g.generateEmail()},
			FormField{Name: "comment", Value: g.generateText(50)},
		)
	case FormTypeContact:
		fields = append(fields,
			FormField{Name: "name", Value: g.generateUsername()},
			FormField{Name: "email", Value: g.generateEmail()},
			FormField{Name: "subject", Value: g.generateSubject()},
			FormField{Name: "message", Value: g.generateText(100)},
		)
	case FormTypeSearch:
		fields = append(fields,
			FormField{Name: "q", Value: g.generateSearchQuery()},
		)
	default:
		fields = append(fields,
			FormField{Name: "username", Value: g.generateUsername()},
			FormField{Name: "data", Value: g.generateText(30)},
		)
	}

	remaining := g.FieldCount - len(fields)
	for i := 0; i < remaining; i++ {
		fields = append(fields, FormField{
			Name:  fmt.Sprintf("field%d", i),
			Value: fmt.Sprintf("value%d", rand.Intn(10000)),
		})
	}

	return fields
}

// EncodeURLEncoded encodes fields as application/x-www-form-urlencoded.
func (g *FormDataGenerator) EncodeURLEncoded(fields []FormField) []byte {
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = url.QueryEscape(f.Name) + "=" + url.QueryEscape(f.Value)
	}
	return []byte(strings.Join(parts, "&"))
}

// EncodeJSON encodes fields as JSON.
func (g *FormDataGenerator) EncodeJSON(fields []FormField) []byte {
	data := make(map[string]string, len(fields))
	for _, f := range fields {
		data[f.Name] = f.Value
	}
	encoded, _ := json.Marshal(data)
	return encoded
}

// EncodeMultipart encodes fields as multipart/form-data.
func (g *FormDataGenerator) EncodeMultipart(fields []FormField) ([]byte, string) {
	boundary := g.generateBoundary()
	var sb strings.Builder

	for _, f := range fields {
		sb.WriteString("--")
		sb.WriteString(boundary)
		sb.WriteString("\r\n")
		sb.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"%s\"\r\n\r\n", f.Name))
		sb.WriteString(f.Value)
		sb.WriteString("\r\n")
	}

	sb.WriteString("--")
	sb.WriteString(boundary)
	sb.WriteString("--\r\n")

	return []byte(sb.String()), boundary
}

// Encode encodes fields based on generator settings.
func (g *FormDataGenerator) Encode(fields []FormField) ([]byte, string) {
	if g.UseJSON {
		return g.EncodeJSON(fields), "application/json"
	}
	if g.UseMultipart {
		data, boundary := g.EncodeMultipart(fields)
		return data, "multipart/form-data; boundary=" + boundary
	}
	return g.EncodeURLEncoded(fields), "application/x-www-form-urlencoded"
}

func (g *FormDataGenerator) generateUsername() string {
	return fmt.Sprintf("user%d", rand.Intn(9000)+1000)
}

func (g *FormDataGenerator) generateEmail() string {
	return fmt.Sprintf("user%d@example.com", rand.Intn(9000)+1000)
}

func (g *FormDataGenerator) generatePassword() string {
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%"
	length := rand.Intn(8) + 8
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

func (g *FormDataGenerator) generateText(wordCount int) string {
	words := []string{
		"test", "sample", "data", "content", "message",
		"hello", "world", "request", "submit", "form",
		"input", "value", "text", "string", "payload",
	}

	selected := make([]string, wordCount)
	for i := 0; i < wordCount; i++ {
		selected[i] = words[rand.Intn(len(words))]
	}
	return strings.Join(selected, " ")
}

func (g *FormDataGenerator) generateSubject() string {
	subjects := []string{
		"Inquiry about services",
		"Support request",
		"General question",
		"Feedback",
		"Technical issue",
	}
	return subjects[rand.Intn(len(subjects))]
}

func (g *FormDataGenerator) generateSearchQuery() string {
	queries := []string{
		"how to", "best practices", "tutorial",
		"guide", "example", "documentation",
	}
	return queries[rand.Intn(len(queries))] + " " + fmt.Sprintf("%d", rand.Intn(100))
}

func (g *FormDataGenerator) generateBoundary() string {
	hash := md5.Sum([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return "----WebKitFormBoundary" + hex.EncodeToString(hash[:])[:16]
}

// RandomFormEndpoint returns a random form submission endpoint.
func RandomFormEndpoint() string {
	return FormEndpoints[rand.Intn(len(FormEndpoints))]
}

// RandomFormReferer returns a random referrer URL for the given form type.
func RandomFormReferer(formType FormType) string {
	referers := FormReferers[formType]
	if len(referers) == 0 {
		referers = FormReferers[FormTypeLogin]
	}
	return referers[rand.Intn(len(referers))]
}

// RandomContentType returns a random content type.
func RandomContentType() string {
	return ContentTypes[rand.Intn(len(ContentTypes))]
}
