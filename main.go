package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sashabaranov/go-openai"
	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/proto"
)

// --- Global Variables ---
var (
	clients      = make(map[string]*whatsmeow.Client)
	clientsMutex sync.RWMutex
	container    *sqlstore.Container
	db           *sql.DB
	openaiClient *openai.Client
	startTime    = time.Now()
)

// --- Helpers ---
func JSONResponse(c *gin.Context, code int, status, message string, data interface{}) {
	c.JSON(code, gin.H{"code": code, "status": status, "message": message, "data": data})
}

func generateRandomToken() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func getJWTKey() []byte {
	key := os.Getenv("JWT_SECRET")
	if key == "" {
		return []byte("super_secret_key_123")
	}
	return []byte(key)
}

func generateToken(username string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(time.Hour * 72).Unix(),
	})
	return token.SignedString(getJWTKey())
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func checkPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func sendEmailResend(toEmail, token string) error {
	apiKey := os.Getenv("RESEND_API_KEY")
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "http://localhost:8080"
	}
	url := "https://api.resend.com/emails"
	verificationLink := fmt.Sprintf("%s/api/v1/auth/verify-email?token=%s", appURL, token)

	payload := map[string]interface{}{
		"from":    os.Getenv("EMAIL_FROM"),
		"to":      []string{toEmail},
		"subject": "Verifikasi Email Akun WA Gateway",
		"html":    fmt.Sprintf("<strong>Halo!</strong><p>Klik link di bawah ini untuk verifikasi email Anda:</p><a href='%s'>Verifikasi Akun Sekarang</a>", verificationLink),
	}

	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(jsonPayload)))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to send email, status: %d", resp.StatusCode)
	}
	return nil
}

func sendWebhook(data interface{}) {
	webhookURL := os.Getenv("WEBHOOK_URL")
	if webhookURL == "" {
		return
	}

	jsonData, _ := json.Marshal(data)
	req, err := http.NewRequest("POST", webhookURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Secret", os.Getenv("WEBHOOK_SECRET"))

	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

func getSystemPrompt(username string) string {
	var prompt sql.NullString
	err := db.QueryRow("SELECT system_prompt FROM users WHERE username = ?", username).Scan(&prompt)
	if err == nil && prompt.Valid && prompt.String != "" {
		return prompt.String
	}
	return os.Getenv("DEFAULT_SYSTEM_PROMPT")
}

func getAIResponse(username, jid, userInput string) string {
	sysPrompt := getSystemPrompt(username)

	resp, err := openaiClient.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: os.Getenv("CUSTOM_MODEL"),
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userInput},
		},
	})
	if err != nil {
		fmt.Printf("AI Error: %v\n", err)
		return "Maaf, asisten sedang sibuk. Silakan coba lagi nanti."
	}
	return resp.Choices[0].Message.Content
}

// --- MIDDLEWARE ---
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			JSONResponse(c, 401, "error", "Authorization header required", nil)
			c.Abort()
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return getJWTKey(), nil
		})
		if err != nil {
			JSONResponse(c, 401, "error", "Invalid token", err.Error())
			c.Abort()
			return
		}
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			c.Set("username", claims["username"])
			c.Next()
		} else {
			JSONResponse(c, 401, "error", "Invalid token claims", nil)
			c.Abort()
		}
	}
}

// --- HANDLERS ---
func handleRegister(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required,min=4,max=20"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6,max=50"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONResponse(c, 400, "error", "Validation failed", err.Error())
		return
	}

	// Validasi Username Admin
	lowerUsername := strings.ToLower(req.Username)
	if lowerUsername == "admin" || strings.HasPrefix(lowerUsername, "admin") {
		JSONResponse(c, 400, "error", "Username tidak diperbolehkan (reserved word)", nil)
		return
	}

	var exists int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ? OR email = ?", req.Username, req.Email).Scan(&exists)
	if exists > 0 {
		JSONResponse(c, 409, "error", "Username or Email already registered", nil)
		return
	}

	token := generateRandomToken()
	hashed, _ := hashPassword(req.Password)
	_, err := db.Exec("INSERT INTO users (username, email, password, verification_token) VALUES (?, ?, ?, ?)", req.Username, req.Email, hashed, token)
	if err != nil {
		JSONResponse(c, 500, "error", "Failed to register user", err.Error())
		return
	}

	_ = sendEmailResend(req.Email, token)

	JSONResponse(c, 201, "success", "User registered. Please check your email for verification.", nil)
}

func handleVerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		JSONResponse(c, 400, "error", "Token is required", nil)
		return
	}

	var username string
	err := db.QueryRow("SELECT username FROM users WHERE verification_token = ?", token).Scan(&username)
	if err != nil {
		JSONResponse(c, 400, "error", "Invalid or expired token", nil)
		return
	}

	_, err = db.Exec("UPDATE users SET is_verified = 1, verification_token = NULL WHERE username = ?", username)
	if err != nil {
		JSONResponse(c, 500, "error", "Failed to verify email", nil)
		return
	}

	c.Writer.Header().Set("Content-Type", "text/html")
	c.Writer.Write([]byte("<h1>Email Terverifikasi!</h1><p>Akun Anda sudah aktif, silakan login melalui aplikasi.</p>"))
}

func handleLogin(c *gin.Context) {
	var req struct {
		Identifier string `json:"identifier" binding:"required"`
		Password   string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONResponse(c, 400, "error", "Identifier and password are required", nil)
		return
	}

	var username, hashedPassword string
	var isVerified int
	err := db.QueryRow("SELECT username, password, is_verified FROM users WHERE username = ? OR email = ?", req.Identifier, req.Identifier).Scan(&username, &hashedPassword, &isVerified)
	if err != nil || !checkPassword(req.Password, hashedPassword) {
		JSONResponse(c, 401, "error", "Invalid credentials", nil)
		return
	}

	if isVerified == 0 {
		JSONResponse(c, 403, "error", "Email belum terverifikasi. Silakan cek inbox email Anda.", nil)
		return
	}

	token, _ := generateToken(username)
	JSONResponse(c, 200, "success", "Login successful", gin.H{"token": token})
}

func handleUpdatePrompt(c *gin.Context) {
	username := c.MustGet("username").(string)
	var req struct {
		Prompt string `json:"prompt" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONResponse(c, 400, "error", "Prompt is required", nil)
		return
	}

	_, err := db.Exec("UPDATE users SET system_prompt = ? WHERE username = ?", req.Prompt, username)
	if err != nil {
		JSONResponse(c, 500, "error", "Failed to update prompt", nil)
		return
	}
	JSONResponse(c, 200, "success", "System prompt updated successfully", nil)
}

func handleConnect(c *gin.Context) {
	username := c.MustGet("username").(string)
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	if cli, ok := clients[username]; ok && cli.IsConnected() {
		JSONResponse(c, 200, "connected", "Already connected", gin.H{"jid": cli.Store.ID.String()})
		return
	}

	device, _ := container.GetFirstDevice(context.Background())
	if device == nil {
		device = container.NewDevice()
	}

	client := whatsmeow.NewClient(device, nil)
	client.AddEventHandler(eventHandler(client, username))

	if client.Store.ID == nil {
		qrChan, _ := client.GetQRChannel(context.Background())
		if err := client.Connect(); err != nil {
			JSONResponse(c, 500, "error", "Failed to connect", nil)
			return
		}
		evt := <-qrChan
		if evt.Event == "code" {
			JSONResponse(c, 200, "scan_required", "Please scan QR code", gin.H{"qr_code": evt.Code})
			go func() {
				for range qrChan {
				}
			}()
		}
	} else {
		if err := client.Connect(); err != nil {
			JSONResponse(c, 500, "error", "Failed to reconnect", nil)
			return
		}
		JSONResponse(c, 200, "connected", "Connected", gin.H{"jid": client.Store.ID.String()})
	}
	clients[username] = client
}

func handleStatus(c *gin.Context) {
	username := c.MustGet("username").(string)
	clientsMutex.RLock()
	cli, ok := clients[username]
	clientsMutex.RUnlock()

	if !ok || !cli.IsConnected() {
		JSONResponse(c, 200, "disconnected", "Not connected", nil)
		return
	}
	JSONResponse(c, 200, "connected", "Connected", gin.H{"jid": cli.Store.ID.String()})
}

func handleSend(c *gin.Context) {
	username := c.MustGet("username").(string)
	var req struct {
		To      string `json:"to" binding:"required"`
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONResponse(c, 400, "error", "To and Message are required", nil)
		return
	}

	clientsMutex.RLock()
	cli, ok := clients[username]
	clientsMutex.RUnlock()

	if !ok || !cli.IsConnected() {
		JSONResponse(c, 503, "error", "WhatsApp not connected", nil)
		return
	}

	targetJID, _ := types.ParseJID(req.To + "@s.whatsapp.net")
	_, err := cli.SendMessage(context.Background(), targetJID, &waE2E.Message{Conversation: proto.String(req.Message)})
	if err != nil {
		JSONResponse(c, 500, "error", "Failed to send message", nil)
		return
	}
	JSONResponse(c, 200, "success", "Message sent", nil)
}

func handleWebhook(c *gin.Context) {
	// Verifikasi Secret
	secret := c.GetHeader("X-Webhook-Secret")
	if secret != os.Getenv("WEBHOOK_SECRET") {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(400, gin.H{"error": "Invalid data"})
		return
	}

	// Untuk sementara, kita log ke console server
	fmt.Printf("\n--- WEBHOOK RECEIVED ---\n")
	fmt.Printf("Event: %v\n", data["event"])
	fmt.Printf("From: %v\n", data["from"])
	fmt.Printf("Message: %v\n", data["message"])
	fmt.Printf("AI Reply: %v\n", data["reply"])
	fmt.Printf("------------------------\n\n")

	c.JSON(200, gin.H{"status": "received"})
}

func eventHandler(client *whatsmeow.Client, username string) func(interface{}) {
	return func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Connected:
			fmt.Printf("[%s] WhatsApp Connected!\n", username)
		case *events.Message:
			if !v.Info.IsFromMe && v.Message.GetConversation() != "" {
				go func() {
					ctx := context.Background()
					jid := v.Info.Sender.ToNonAD().String()
					userInput := v.Message.GetConversation()

					time.Sleep(2 * time.Second)
					client.MarkRead(ctx, []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Sender, v.Info.Sender)
					client.SendChatPresence(ctx, v.Info.Sender, types.ChatPresenceComposing, types.ChatPresenceMediaText)

					aiResponse := getAIResponse(username, jid, userInput)

					time.Sleep(3 * time.Second)
					client.SendChatPresence(ctx, v.Info.Sender, types.ChatPresencePaused, types.ChatPresenceMediaText)

					client.SendMessage(ctx, v.Info.Sender, &waE2E.Message{Conversation: proto.String(aiResponse)})

					sendWebhook(map[string]interface{}{
						"event":   "message_replied",
						"from":    jid,
						"message": userInput,
						"reply":   aiResponse,
						"time":    time.Now().Format(time.RFC3339),
					})
				}()
			}
		}
	}
}

// --- CORE ---
func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", "file:wa_gateway.db?_journal_mode=WAL")
	if err != nil {
		return err
	}
	db.Exec("PRAGMA journal_mode=WAL;")
	db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		username TEXT UNIQUE, 
		email TEXT UNIQUE,
		password TEXT,
		is_verified INTEGER DEFAULT 0,
		verification_token TEXT,
		system_prompt TEXT
	)`)
	return nil
}

func handleMe(c *gin.Context) {
	username := c.MustGet("username").(string)
	var email string
	err := db.QueryRow("SELECT email FROM users WHERE username = ?", username).Scan(&email)
	if err != nil {
		JSONResponse(c, 404, "error", "User not found", nil)
		return
	}
	JSONResponse(c, 200, "success", "User profile retrieved", gin.H{
		"username": username,
		"email":    email,
	})
}

func handleForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONResponse(c, 400, "error", "Valid email is required", nil)
		return
	}

	var username string
	err := db.QueryRow("SELECT username FROM users WHERE email = ?", req.Email).Scan(&username)
	if err != nil {
		// Untuk keamanan, jangan beri tahu jika email tidak ada, tapi tetap beri respon sukses
		JSONResponse(c, 200, "success", "If your email is registered, you will receive a reset link.", nil)
		return
	}

	token := generateRandomToken()
	_, err = db.Exec("UPDATE users SET verification_token = ? WHERE email = ?", token, req.Email)
	if err != nil {
		JSONResponse(c, 500, "error", "Failed to process request", nil)
		return
	}

	// Kirim Email
	apiKey := os.Getenv("RESEND_API_KEY")
	appURL := os.Getenv("APP_URL")
	resetLink := fmt.Sprintf("%s/api/v1/auth/reset-password?token=%s", appURL, token)

	payload := map[string]interface{}{
		"from":    os.Getenv("EMAIL_FROM"),
		"to":      []string{req.Email},
		"subject": "Reset Password Akun WA Gateway",
		"html":    fmt.Sprintf("<strong>Halo!</strong><p>Anda meminta untuk reset password. Klik link di bawah ini:</p><a href='%s'>Reset Password Sekarang</a><p>Jika Anda tidak meminta ini, abaikan saja email ini.</p>", resetLink),
	}

	jsonPayload, _ := json.Marshal(payload)
	reqHttp, _ := http.NewRequest("POST", "https://api.resend.com/emails", strings.NewReader(string(jsonPayload)))
	reqHttp.Header.Set("Authorization", "Bearer "+apiKey)
	reqHttp.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, _ := httpClient.Do(reqHttp)
	if resp != nil {
		resp.Body.Close()
	}

	JSONResponse(c, 200, "success", "If your email is registered, you will receive a reset link.", nil)
}

func handleResetPassword(c *gin.Context) {
	if c.Request.Method == "GET" {
		token := c.Query("token")
		// Tampilkan form sederhana atau arahkan ke frontend
		c.Writer.Header().Set("Content-Type", "text/html")
		html := fmt.Sprintf(`
			<html>
			<body>
				<h2>Reset Password</h2>
				<form action="/api/v1/auth/reset-password" method="POST">
					<input type="hidden" name="token" value="%s">
					<input type="password" name="password" placeholder="Password Baru" required minlength="6">
					<button type="submit">Ganti Password</button>
				</form>
			</body>
			</html>
		`, token)
		c.Writer.Write([]byte(html))
		return
	}

	// Handle POST (form submission)
	token := c.PostForm("token")
	newPassword := c.PostForm("password")

	if token == "" || newPassword == "" {
		// Coba bind JSON jika bukan dari form HTML (untuk API frontend)
		var req struct {
			Token    string `json:"token"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err == nil {
			token = req.Token
			newPassword = req.Password
		}
	}

	if token == "" || len(newPassword) < 6 {
		JSONResponse(c, 400, "error", "Invalid token or password too short", nil)
		return
	}

	hashed, _ := hashPassword(newPassword)
	res, err := db.Exec("UPDATE users SET password = ?, verification_token = NULL WHERE verification_token = ?", hashed, token)
	if err != nil {
		JSONResponse(c, 500, "error", "Database error", nil)
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		JSONResponse(c, 400, "error", "Invalid or expired token", nil)
		return
	}

	c.Writer.Header().Set("Content-Type", "text/html")
	c.Writer.Write([]byte("<h1>Password Berhasil Diganti!</h1><p>Silakan login kembali dengan password baru Anda.</p>"))
}

func main() {
	godotenv.Load()
	if err := initDB(); err != nil {
		panic(err)
	}

	// Init AI
	config := openai.DefaultConfig(os.Getenv("CUSTOM_API_KEY"))
	config.BaseURL = os.Getenv("CUSTOM_BASE_URL")
	openaiClient = openai.NewClientWithConfig(config)

	var err error
	dbLog := waLog.Stdout("Database", "ERROR", true)
	container, err = sqlstore.New(context.Background(), "sqlite3", "file:wa_sessions.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}

	r := gin.Default()

	// Middleware CORS
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	rate := limiter.Rate{Period: 1 * time.Minute, Limit: 30}
	limitMiddleware := mgin.NewMiddleware(limiter.New(memory.NewStore(), rate))
	r.Use(limitMiddleware)

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", func(c *gin.Context) {
			JSONResponse(c, 200, "success", "Server is running", gin.H{"uptime": time.Since(startTime).String()})
		})
		auth := v1.Group("/auth")
		{
			auth.POST("/register", handleRegister)
			auth.POST("/login", handleLogin)
			auth.GET("/verify-email", handleVerifyEmail)
			auth.GET("/me", authMiddleware(), handleMe)
			auth.POST("/forgot-password", handleForgotPassword)
			auth.GET("/reset-password", handleResetPassword)
			auth.POST("/reset-password", handleResetPassword)
		}

		v1.GET("/docs", func(c *gin.Context) {
			c.File("API_DOCS.md")
		})

		v1.POST("/webhook/receiver", handleWebhook)

		wa := v1.Group("/whatsapp")
		wa.Use(authMiddleware())
		{
			wa.GET("/sessions/connect", handleConnect)
			wa.GET("/sessions/status", handleStatus)
			wa.POST("/settings/prompt", handleUpdatePrompt)
			wa.POST("/messages/send", handleSend)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}
