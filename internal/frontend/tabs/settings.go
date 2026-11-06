package tabs

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/ardora/stockcontroller/internal/backend/services"
	"github.com/ardora/stockcontroller/internal/frontend"
	"github.com/ardora/stockcontroller/internal/frontend/components"
	"github.com/ardora/stockcontroller/internal/frontend/style"
)

type SettingsTab struct {
	state *frontend.State

	configService *services.ConfigService
	backupService *services.BackupService

	themeDark bool

	twilioKey        string
	targetEmail      string
	entryPrice       string
	businessName     string
	businessDocument string
	businessAddress  string

	msg     string
	msgOK   bool
	saving  bool
	backing bool
}

func NewSettingsTab(s *frontend.State, config *services.ConfigService, backup *services.BackupService) *SettingsTab {
	t := &SettingsTab{state: s, configService: config, backupService: backup}
	t.loadConfig()
	return t
}

func (t *SettingsTab) loadConfig() {
	go func() {
		dark, _ := t.configService.Get(context.Background(), "theme_dark")
		key, _ := t.configService.Get(context.Background(), "twilio_key")
		email, _ := t.configService.Get(context.Background(), "target_email")
		entryPrice, _ := t.configService.EntryPriceCents(context.Background())
		businessName, _ := t.configService.Get(context.Background(), "business_name")
		businessDocument, _ := t.configService.Get(context.Background(), "business_document")
		businessAddress, _ := t.configService.Get(context.Background(), "business_address")

		t.state.Window.Update(func() {
			t.themeDark = dark == "true"
			t.twilioKey = key
			t.targetEmail = email
			t.entryPrice = services.FormatBRLCents(entryPrice)
			t.businessName = businessName
			t.businessDocument = businessDocument
			t.businessAddress = businessAddress
		})
	}()
}

func (t *SettingsTab) View(c *ui.Context) {
	components.Page(c, func() {
		components.PageHeader(c, "PREFERÊNCIAS", "Configurações", "Personalize a aparência e configure o envio do relatório diário.", nil)

		if t.msg != "" {
			components.FeedbackBanner(c, t.msg, feedbackTone(t.msgOK))
		}

		ui.Row(c).Gap(16).Wrap().AlignItems(ui.Start).Children(func() {
			components.Panel(c, "Aparência", "Escolha como a interface será exibida.", 280, func() {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Switch(c, &t.themeDark).Disabled(t.saving || t.backing)
					ui.Column(c).Gap(2).Grow(1).Children(func() {
						ui.Text(c, "Modo escuro").FontWeight(700)
						ui.Text(c, "A alteração será aplicada na próxima inicialização.").FontSize(12).TextColor(style.FromContext(c).Muted)
					})
				})
			})

			components.Panel(c, "Backup e e-mail", "Gere o CSV diário e envie para o endereço configurado.", 320, func() {
				ui.Field(c, "Chave da API Twilio/SendGrid", func() {
					ui.TextInput(c, &t.twilioKey).Disabled(t.saving || t.backing)
				})
				ui.Text(c, "A chave fica armazenada localmente neste computador.").FontSize(12).TextColor(style.FromContext(c).Muted)
				ui.Field(c, "E-mail de destino", func() {
					ui.TextInput(c, &t.targetEmail).Disabled(t.saving || t.backing)
				})
			})

			components.Panel(c, "Atendimento e recibo", "Defina a entrada e os dados exibidos no recibo não fiscal.", 320, func() {
				ui.Field(c, "Preço padrão da entrada *", func() {
					ui.TextInput(c, &t.entryPrice).Disabled(t.saving || t.backing)
				})
				ui.Text(c, "O valor é copiado para a comanda no check-in.").FontSize(12).TextColor(style.FromContext(c).Muted)
				ui.Field(c, "Nome do estabelecimento", func() {
					ui.TextInput(c, &t.businessName).Disabled(t.saving || t.backing)
				})
				ui.Field(c, "Documento", func() {
					ui.TextInput(c, &t.businessDocument).Disabled(t.saving || t.backing)
				})
				ui.Field(c, "Endereço", func() {
					ui.TextInput(c, &t.businessAddress).Disabled(t.saving || t.backing)
				})
			})
		})

		components.Panel(c, "Ações", "Salve as preferências ou execute o relatório imediatamente.", 0, func() {
			ui.Row(c).Gap(12).Wrap().Children(func() {
				if ui.PrimaryButton(c, busyLabel(t.saving, "Salvar configurações")).Disabled(t.saving || t.backing).Clicked() {
					t.saveConfig()
				}
				if ui.Button(c, busyLabel(t.backing, "Gerar CSV e enviar agora")).Disabled(t.saving || t.backing).Clicked() {
					t.triggerBackup()
				}
			})
		})
	})
}

func (t *SettingsTab) saveConfig() {
	if t.saving || t.backing {
		return
	}
	entryPrice, err := services.ParseBRLCents(t.entryPrice)
	if err != nil {
		t.msgOK = false
		t.msg = "Informe um preço de entrada válido: " + err.Error()
		return
	}
	t.saving = true
	t.msg = ""
	go func() {
		darkValue := "false"
		if t.themeDark {
			darkValue = "true"
		}

		ctx := context.Background()
		err := t.configService.Set(ctx, "theme_dark", darkValue)
		if err == nil {
			err = t.configService.Set(ctx, "twilio_key", strings.TrimSpace(t.twilioKey))
		}
		if err == nil {
			err = t.configService.Set(ctx, "target_email", strings.TrimSpace(t.targetEmail))
		}
		if err == nil {
			err = t.configService.SetEntryPriceCents(ctx, entryPrice)
		}
		if err == nil {
			err = t.configService.Set(ctx, "business_name", strings.TrimSpace(t.businessName))
		}
		if err == nil {
			err = t.configService.Set(ctx, "business_document", strings.TrimSpace(t.businessDocument))
		}
		if err == nil {
			err = t.configService.Set(ctx, "business_address", strings.TrimSpace(t.businessAddress))
		}

		t.state.Window.Update(func() {
			t.saving = false
			if err != nil {
				t.msgOK = false
				t.msg = "Não foi possível salvar as configurações: " + err.Error()
				return
			}
			t.msgOK = true
			t.entryPrice = services.FormatBRLCents(entryPrice)
			t.msg = "Configurações salvas. Reinicie o aplicativo para aplicar a aparência."
		})
	}()
}

func (t *SettingsTab) triggerBackup() {
	email := strings.TrimSpace(t.targetEmail)
	key := strings.TrimSpace(t.twilioKey)
	if email == "" {
		t.msgOK = false
		t.msg = "Informe o e-mail de destino antes de enviar."
		return
	}
	if key == "" {
		t.msgOK = false
		t.msg = "Informe a chave da API antes de enviar."
		return
	}
	if t.saving || t.backing {
		return
	}

	t.backing = true
	t.msg = ""
	go func() {
		path, err := t.backupService.GenerateCSV(context.Background(), "")
		if err == nil {
			err = t.backupService.SendEmail(context.Background(), email, key, "Relatório Diário Ardora", "Segue anexo o relatório", path)
		}
		t.state.Window.Update(func() {
			t.backing = false
			if err != nil {
				t.msgOK = false
				t.msg = "O relatório não pôde ser enviado: " + err.Error()
				return
			}
			t.msgOK = true
			t.msg = "Backup gerado e enviado com sucesso."
		})
	}()
}
