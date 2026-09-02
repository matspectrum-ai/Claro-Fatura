package httpapi

import(
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)
//go:embed web
var embeddedWeb embed.FS
var webFS fs.FS
func init(){var err error;webFS,err=fs.Sub(embeddedWeb,"web");if err!=nil{panic(err)}}
func(s *Server)home(w http.ResponseWriter,r *http.Request){if r.URL.Path!="/"{http.NotFound(w,r);return};s.serveWebFile(w,r,"index.html",false)}
func(s *Server)adminPage(w http.ResponseWriter,r *http.Request){s.serveWebFile(w,r,"admin.html",false)}
func(s *Server)adminClientesRedirect(w http.ResponseWriter,r *http.Request){http.Redirect(w,r,"/admin/faturas",http.StatusFound)}
func(s *Server)adminActivityPage(w http.ResponseWriter,r *http.Request){s.serveWebFile(w,r,"admin-activity.html",false)}
func(s *Server)adminGatewaysPage(w http.ResponseWriter,r *http.Request){s.serveWebFile(w,r,"admin-gateways.html",false)}
func(s *Server)invoicePage(w http.ResponseWriter,r *http.Request){phone:=strings.TrimSpace(r.PathValue("telefone"));if phone==""||strings.Contains(phone,"/"){http.NotFound(w,r);return};s.serveWebFile(w,r,"fatura.html",false)}
func(s *Server)asset(w http.ResponseWriter,r *http.Request){name:=strings.TrimSpace(r.PathValue("path"));if name==""||strings.Contains(name,"..")||strings.HasPrefix(name,"/"){http.NotFound(w,r);return};s.serveWebFile(w,r,path.Join("assets",name),true)}
func(s *Server)favicon(w http.ResponseWriter,r *http.Request){s.serveWebFile(w,r,"favicon.png",true)}
func(s *Server)robots(w http.ResponseWriter,r *http.Request){s.serveWebFile(w,r,"robots.txt",true)}
func(s *Server)serveWebFile(w http.ResponseWriter,r *http.Request,name string,immutable bool){data,err:=fs.ReadFile(webFS,name);if err!=nil{http.NotFound(w,r);return};contentType:=mime.TypeByExtension(path.Ext(name));if contentType==""{contentType="application/octet-stream"};if strings.HasSuffix(name,".html"){contentType="text/html; charset=utf-8"};if strings.HasSuffix(name,".js"){contentType="text/javascript; charset=utf-8"};if strings.HasSuffix(name,".css"){contentType="text/css; charset=utf-8"};w.Header().Set("Content-Type",contentType);if immutable{w.Header().Set("Cache-Control","public, max-age=86400")}else{w.Header().Set("Cache-Control","no-cache")};w.Header().Set("Content-Length",itoa(len(data)));w.WriteHeader(200);_,_=w.Write(data)}
func itoa(n int)string{if n==0{return "0"};var b [24]byte;i:=len(b);for n>0{i--;b[i]=byte('0'+n%10);n/=10};return string(b[i:])}
