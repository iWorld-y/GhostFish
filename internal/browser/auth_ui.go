package browser

import (
	"github.com/egoist/mygo/ui"
)

// authDialog 显示站点保护空间，并用遮挡输入接收密码。
func (a *Browser) authDialog(c *ui.Context, p *authPrompt) {
	ui.Scroll(c.Key(p)).Grow(1).Children(func() {
		ui.Column(c).FillWidth().Padding(20).Gap(12).Background(c.Theme().Background).Children(func() {
			ui.Text(c, "网站需要身份验证").FontSize(18).Bold()
			ui.Text(c, "主机："+authHostLabel(p)).Wrap()
			if p.need.Realm != "" {
				realm := []rune(p.need.Realm)
				if len(realm) > 100 {
					realm = append(realm[:100], '…')
				}
				ui.Text(c, "认证范围："+string(realm)).Wrap()
			}
			ui.TextInput(c.Key("username"), &p.username).Label("用户名").Placeholder("用户名").Disabled(p.busy)
			ui.TextInput(c.Key("password"), &p.password).Label("密码").Placeholder("密码").Password().Disabled(p.busy).
				OnSubmit(func() { a.submitAuth(p) })
			if p.errorText != "" {
				ui.Text(c, p.errorText).TextColor(c.Theme().Danger).Wrap()
			} else if p.busy {
				ui.Text(c, "正在验证…").TextColor(c.Theme().TextMuted)
			}
			ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
				ui.Button(c, "取消").OnClick(func() { a.cancelTabAuth(p.tab) })
				ui.PrimaryButton(c, "登录").Disabled(p.busy).OnClick(func() { a.submitAuth(p) })
			})
		})
	})
}
