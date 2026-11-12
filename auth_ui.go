package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// openAuthWindow 使用独立的不透明窗口，避免主窗口隐藏或穿透时无法输入。
func (a *app) openAuthWindow(p *authPrompt) {
	if !a.validAuthPrompt(p) || p.window != nil {
		return
	}
	p.window = mygo.NewWindow(mygo.WindowOptions{
		Title: "网站需要身份验证", Width: 420, Height: 340,
		MinWidth: 320, MinHeight: 300, AlwaysOnTop: true, Opacity: 1,
		Content: ui.View(func(c *ui.Context) { a.authDialog(c, p) }),
	})
	win := p.window
	win.OnClosed(func() {
		if p.window == win {
			p.window = nil
		}
		if p.tab.authPrompt == p {
			a.cancelTabAuth(p.tab)
		}
	})
	win.Show()
	mygo.App.Focus()
	win.Focus()
}

// authDialog 显示站点保护空间，并用遮挡输入接收密码。
func (a *app) authDialog(c *ui.Context, p *authPrompt) {
	ui.Column(c).Fill().Padding(20).Gap(12).Background(c.Theme().Background).Children(func() {
		ui.Text(c, "网站需要身份验证").FontSize(18).Bold()
		ui.Text(c, "主机："+authHostLabel(p)).Wrap()
		if p.need.Realm != "" {
			realm := []rune(p.need.Realm)
			if len(realm) > 100 {
				realm = append(realm[:100], '…')
			}
			ui.Text(c, "认证范围："+string(realm)).Wrap()
		}
		ui.TextInput(c.Key("username"), &p.username).Label("用户名").Placeholder("用户名").AutoFocus().Disabled(p.busy)
		ui.TextInput(c.Key("password"), &p.password).Label("密码").Placeholder("密码").Password().Disabled(p.busy).
			OnSubmit(func() { a.submitAuth(p) })
		if p.errorText != "" {
			ui.Text(c, p.errorText).TextColor(c.Theme().Danger).Wrap()
		} else if p.busy {
			ui.Text(c, "正在验证…").TextColor(c.Theme().TextMuted)
		}
		ui.Box(c).Grow(1)
		ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
			ui.Button(c, "取消").OnClick(func() { a.cancelTabAuth(p.tab) })
			ui.PrimaryButton(c, "登录").Disabled(p.busy).OnClick(func() { a.submitAuth(p) })
		})
	})
}
