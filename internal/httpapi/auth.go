package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	authsvc "github.com/matspectrum-ai/Claro-Fatura/internal/auth"
)

const (
	accessCookieName  = "cf_admin_access"
	refreshCookieName = "cf_admin_refresh"
)

type AdminAuth interface {
	SignIn(context.Context, string, string) (authsvc.Session, error)
	SignUp(context.Context, string, string, string, string) (authsvc.Session, bool, error)
	RequireAdmin(context.Context, string) (authsvc.User, error)
	Refresh(context.Context, string) (authsvc.Session, error)
	SendRecovery(context.Context, string, string) error
	UpdatePassword(context.Context, string, string) error
	Logout(context.Context, string) error
}

func (s *Server) authPage(w http.ResponseWriter, r *http.Request) { s.serveWebFile(w, r, "auth.html", false) }
func (s *Server) forgotPasswordPage(w http.ResponseWriter, r *http.Request) { s.serveWebFile(w, r, "forgot-password.html", false) }
func (s *Server) resetPasswordPage(w http.ResponseWriter, r *http.Request) { s.serveWebFile(w, r, "reset-password.html", false) }

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.deps.Auth == nil { writeJSON(w, http.StatusServiceUnavailable, map[string]string{"erro":"Autenticação indisponível."}); return }
	var body struct{ Email string `json:"email"`; Password string `json:"senha"` }
	if decodeJSON(r,&body)!=nil || strings.TrimSpace(body.Email)=="" || body.Password=="" { writeJSON(w,http.StatusBadRequest,map[string]string{"erro":"Dados inválidos."}); return }
	session,err:=s.deps.Auth.SignIn(r.Context(),body.Email,body.Password)
	if err!=nil { switch { case errors.Is(err,authsvc.ErrInvalidCredentials): writeJSON(w,http.StatusUnauthorized,map[string]string{"erro":"Verifique o e-mail e a senha."}); case errors.Is(err,authsvc.ErrNotAdmin): clearAdminCookies(w,r); writeJSON(w,http.StatusForbidden,map[string]string{"erro":"Esta conta não tem permissão de administrador."}); default: s.logger.Error("admin login failed","error",err); writeJSON(w,http.StatusBadGateway,map[string]string{"erro":"Não foi possível entrar agora."}) }; return }
	setAdminCookies(w,r,session); writeJSON(w,http.StatusOK,map[string]bool{"ok":true})
}
func (s *Server) signup(w http.ResponseWriter,r *http.Request){ if s.deps.Auth==nil{writeJSON(w,503,map[string]string{"erro":"Autenticação indisponível."});return};var body struct{Name string `json:"nome"`;Email string `json:"email"`;Password string `json:"senha"`};if decodeJSON(r,&body)!=nil||strings.TrimSpace(body.Name)==""||strings.TrimSpace(body.Email)==""||len(body.Password)<6{writeJSON(w,400,map[string]string{"erro":"Dados inválidos."});return};session,confirmation,err:=s.deps.Auth.SignUp(r.Context(),body.Name,body.Email,body.Password,requestBaseURL(r,s.deps.SiteURL));if err!=nil{if errors.Is(err,authsvc.ErrNotAdmin){clearAdminCookies(w,r);writeJSON(w,403,map[string]string{"erro":"Esta conta não tem permissão de administrador."});return};s.logger.Error("admin signup failed","error",err);writeJSON(w,400,map[string]string{"erro":err.Error()});return};if confirmation{writeJSON(w,200,map[string]any{"ok":true,"confirmacao_email":true});return};setAdminCookies(w,r,session);writeJSON(w,200,map[string]any{"ok":true,"confirmacao_email":false}) }
func (s *Server) authMe(w http.ResponseWriter,r *http.Request){user,ok:=s.requireAdmin(w,r);if !ok{return};writeJSON(w,200,map[string]any{"id":user.ID,"email":user.Email,"admin":true})}
func (s *Server) logout(w http.ResponseWriter,r *http.Request){if s.deps.Auth!=nil{if c,err:=r.Cookie(accessCookieName);err==nil{_=s.deps.Auth.Logout(r.Context(),c.Value)}};clearAdminCookies(w,r);w.WriteHeader(http.StatusNoContent)}
func (s *Server) recoverPassword(w http.ResponseWriter,r *http.Request){if s.deps.Auth==nil{writeJSON(w,503,map[string]string{"erro":"Autenticação indisponível."});return};var body struct{Email string `json:"email"`};if decodeJSON(r,&body)!=nil||strings.TrimSpace(body.Email)==""{writeJSON(w,400,map[string]string{"erro":"Informe o e-mail cadastrado."});return};redirect:=requestBaseURL(r,s.deps.SiteURL)+"/reset-password";if err:=s.deps.Auth.SendRecovery(r.Context(),body.Email,redirect);err!=nil{s.logger.Error("password recovery failed","error",err);writeJSON(w,502,map[string]string{"erro":"Não foi possível enviar o link."});return};writeJSON(w,200,map[string]bool{"ok":true})}
func (s *Server) recoverySession(w http.ResponseWriter,r *http.Request){var body struct{Access string `json:"access_token"`;Refresh string `json:"refresh_token"`;ExpiresIn int `json:"expires_in"`};if decodeJSON(r,&body)!=nil||body.Access==""{writeJSON(w,400,map[string]string{"erro":"Link de recuperação inválido."});return};if s.deps.Auth==nil{writeJSON(w,503,map[string]string{"erro":"Autenticação indisponível."});return};if _,err:=s.deps.Auth.RequireAdmin(r.Context(),body.Access);err!=nil{clearAdminCookies(w,r);writeJSON(w,403,map[string]string{"erro":"Acesso restrito."});return};setAdminCookies(w,r,authsvc.Session{AccessToken:body.Access,RefreshToken:body.Refresh,ExpiresIn:body.ExpiresIn});writeJSON(w,200,map[string]bool{"ok":true})}
func (s *Server) updatePassword(w http.ResponseWriter,r *http.Request){if s.deps.Auth==nil{writeJSON(w,503,map[string]string{"erro":"Autenticação indisponível."});return};var body struct{Password string `json:"senha"`};if decodeJSON(r,&body)!=nil||len(body.Password)<6{writeJSON(w,400,map[string]string{"erro":"A senha deve ter pelo menos 6 caracteres."});return};token,ok:=s.validAdminToken(w,r);if !ok{return};if err:=s.deps.Auth.UpdatePassword(r.Context(),token,body.Password);err!=nil{s.logger.Error("password update failed","error",err);writeJSON(w,502,map[string]string{"erro":"Não foi possível redefinir a senha."});return};_=s.deps.Auth.Logout(r.Context(),token);clearAdminCookies(w,r);writeJSON(w,200,map[string]bool{"ok":true})}
func (s *Server) requireAdmin(w http.ResponseWriter,r *http.Request)(authsvc.User,bool){_,user,ok:=s.adminSession(w,r);return user,ok}
func (s *Server) validAdminToken(w http.ResponseWriter,r *http.Request)(string,bool){token,_,ok:=s.adminSession(w,r);return token,ok}
func (s *Server) adminSession(w http.ResponseWriter,r *http.Request)(string,authsvc.User,bool){if s.deps.Auth==nil{writeJSON(w,503,map[string]string{"erro":"Autenticação indisponível."});return "",authsvc.User{},false};access,_:=r.Cookie(accessCookieName);if access!=nil&&access.Value!=""{user,err:=s.deps.Auth.RequireAdmin(r.Context(),access.Value);if err==nil{return access.Value,user,true}};refresh,_:=r.Cookie(refreshCookieName);if refresh==nil||refresh.Value==""{clearAdminCookies(w,r);writeJSON(w,401,map[string]string{"erro":"Sessão expirada."});return "",authsvc.User{},false};session,err:=s.deps.Auth.Refresh(r.Context(),refresh.Value);if err!=nil{clearAdminCookies(w,r);writeJSON(w,401,map[string]string{"erro":"Sessão expirada."});return "",authsvc.User{},false};user,err:=s.deps.Auth.RequireAdmin(r.Context(),session.AccessToken);if err!=nil{clearAdminCookies(w,r);writeJSON(w,403,map[string]string{"erro":"Acesso restrito."});return "",authsvc.User{},false};setAdminCookies(w,r,session);return session.AccessToken,user,true}
func setAdminCookies(w http.ResponseWriter,r *http.Request,sess authsvc.Session){secure:=requestIsSecure(r);max:=sess.ExpiresIn;if max<=0{max=3600};http.SetCookie(w,&http.Cookie{Name:accessCookieName,Value:sess.AccessToken,Path:"/",HttpOnly:true,Secure:secure,SameSite:http.SameSiteLaxMode,MaxAge:max});if sess.RefreshToken!=""{http.SetCookie(w,&http.Cookie{Name:refreshCookieName,Value:sess.RefreshToken,Path:"/",HttpOnly:true,Secure:secure,SameSite:http.SameSiteLaxMode,MaxAge:int((30*24*time.Hour).Seconds())})}}
func clearAdminCookies(w http.ResponseWriter,r *http.Request){secure:=requestIsSecure(r);for _,name:=range []string{accessCookieName,refreshCookieName}{http.SetCookie(w,&http.Cookie{Name:name,Value:"",Path:"/",HttpOnly:true,Secure:secure,SameSite:http.SameSiteLaxMode,MaxAge:-1,Expires:time.Unix(1,0)})}}
func requestIsSecure(r *http.Request)bool{if r.TLS!=nil{return true};return strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"),",")[0]),"https")}
var _=json.Valid
