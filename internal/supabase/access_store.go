package supabase

import "context"

func (c *Client) LogPublicAccess(ctx context.Context, page string) error {
	return c.Insert(ctx, "acessos", map[string]any{
		"pagina":              page,
		"telefone_consultado": nil,
		"sucesso":             false,
		"valor_original":      nil,
		"valor_desconto":      nil,
	})
}
