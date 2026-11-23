// Package i18n translates user facing server messages.
//
// Chinese is the source language: every message passed to Translate is written
// in Chinese and the English catalog maps it to an English equivalent. Unknown
// messages fall back to the source text so a missing translation never breaks a
// response.
package i18n

// Languages supported by the UI.
const (
	Chinese = "zh"
	English = "en"
)

// Normalize returns a supported language code, defaulting to Chinese.
func Normalize(lang string) string {
	if lang == English {
		return English
	}
	return Chinese
}

var englishMessages = map[string]string{
	"服务器开小差了，请稍后再试":                     "Something went wrong. Please try again later.",
	"跨站请求被拒绝":                           "Cross-site request rejected.",
	"请先登录":                              "Please sign in first.",
	"请先以管理员身份登录":                        "Please sign in as an administrator.",
	"请求数据格式不正确":                         "Invalid request body.",
	"用户名或密码不正确":                         "Incorrect username or password.",
	"尝试次数过多，请一分钟后再试":                    "Too many attempts. Try again in a minute.",
	"提交太频繁了，请稍后再试":                      "Too many submissions. Please try again later.",
	"管理员已关闭卡片申请":                        "Card submissions are disabled.",
	"请填写 1-40 个字符的名称":                   "Name must be 1-40 characters.",
	"请填写合法的 http/https 地址":              "Enter a valid http/https URL.",
	"请选择有效的分类":                          "Choose a valid category.",
	"卡片编号不正确":                           "Invalid card ID.",
	"卡片不存在":                             "Card not found.",
	"分类参数不正确":                           "Invalid category parameter.",
	"分类编号不正确":                           "Invalid category ID.",
	"分类不存在":                             "Category not found.",
	"分类名称需为 1-24 个字符":                   "Category name must be 1-24 characters.",
	"分类描述不能超过 200 个字符":                  "Category description cannot exceed 200 characters.",
	"公告编号不正确":                           "Invalid notice ID.",
	"公告不存在":                             "Notice not found.",
	"公告切换间隔需为 3-120 秒":                  "Notice rotation must be 3-120 seconds.",
	"会话有效期需为 1-365 天":                   "Session lifetime must be 1-365 days.",
	"角色不正确":                             "Invalid role.",
	"开关取值不正确":                           "Invalid switch value.",
	"主题色不正确":                            "Invalid accent color.",
	"语言取值不正确":                           "Invalid language.",
	"排序类型不正确":                           "Invalid sort type.",
	"排序数据过多":                            "Too many items to sort.",
	"图片编号不正确":                           "Invalid image ID.",
	"图片不存在":                             "Image not found.",
	"图片不能超过 2MB":                        "Images must be 2 MB or smaller.",
	"图片内容为空":                            "The image is empty.",
	"请选择要上传的图片":                         "Choose an image to upload.",
	"上传内容过大或格式不正确":                      "Upload is too large or has an unsupported format.",
	"仅支持 PNG、JPEG、GIF、WebP、AVIF、ICO 格式": "Only PNG, JPEG, GIF, WebP, AVIF, and ICO are supported.",
	"设置内容过长":                            "Setting value is too long.",
	"LOGO 地址不正确":                        "Invalid logo URL.",
	"密码长度需为 8-72 位":                     "Password must be 8-72 characters.",
	"新密码长度需为 8-72 位":                    "New password must be 8-72 characters.",
	"当前密码不正确":                           "Current password is incorrect.",
	"不能删除当前登录的账号":                       "You cannot delete the account you are signed in with.",
	"至少需要保留一名管理员":                       "At least one administrator must remain.",
	"用户编号不正确":                           "Invalid user ID.",
	"用户不存在":                             "User not found.",
	"用户名需为 3-32 位字母、数字或 _ . -":          "Username must be 3-32 letters, digits, or _ . -",
	"用户名已存在":                            "Username already exists.",
	"显示名称不能超过 32 个字符":                   "Display name cannot exceed 32 characters.",
	"提交成功，等待管理员审核":                      "Submitted. An administrator will review it.",
	"不能将分类移动到自己下面":                      "A category cannot be moved under itself.",
	"不能移动到自己的子分类下面":                     "A category cannot be moved under its own subcategory.",
	"上级分类不存在":                           "Parent category not found.",
	"读取分类失败":                            "Failed to read categories.",
	"分类层级异常":                            "Invalid category hierarchy.",
	"最多支持三级分类":                          "Only three category levels are supported.",
	"最多支持三级分类，请调整上级分类":                  "Only three category levels are supported. Adjust the parent category.",
	"卡片名称需为 1-60 个字符":                   "Card name must be 1-60 characters.",
	"网址必须是合法的 http/https 地址":            "The URL must be a valid http/https address.",
	"描述不能超过 300 个字符":                    "Description cannot exceed 300 characters.",
	"图标类型不正确":                           "Invalid icon type.",
	"图标地址必须是本站上传图片或 http/https 链接":      "The icon must be an uploaded image or an http/https URL.",
	"图标内容过长":                            "Icon content is too long.",
	"请选择存在的分类":                          "Choose an existing category.",
	"卡片状态不正确":                           "Invalid card status.",
	"公告标题需为 1-80 个字符":                   "Notice title must be 1-80 characters.",
	"公告内容不能超过 2000 个字符":                 "Notice content cannot exceed 2000 characters.",
	"公告级别不正确":                           "Invalid notice level.",
	"公告链接必须是 http/https 地址":             "The notice link must be an http/https address.",
	"账号尚未开通，请联系管理员":                     "This account has not been provisioned. Contact an administrator.",
	"账号名称已被占用，请联系管理员":                   "That account name is already taken. Contact an administrator.",
	"目录认证服务暂不可用，请稍后再试":                  "The directory service is unavailable. Please try again later.",
	"外部账号请在身份提供方修改密码":                   "Change the password at your identity provider.",
	"LDAP 地址不能为空":                       "LDAP address is required.",
	"请填写服务账号或用户 DN 模板":                  "Provide a service account or a user DN template.",
	"请填写服务账号密码":                         "The service account password is required.",
	"用户 DN 模板需包含 {username}":            "The user DN template must contain {username}.",
	"使用目录搜索时必须填写 Base DN":               "Base DN is required when searching the directory.",
	"SSO Issuer 不能为空":                   "SSO issuer is required.",
	"SSO Issuer 必须是 http/https 地址":      "SSO issuer must be an http/https URL.",
	"SSO Client ID 不能为空":                "SSO client ID is required.",
	"SSO 回调地址必须是 http/https 地址":         "SSO callback URL must be an http/https URL.",
}

// Translate renders msg in the requested language. Unsupported languages and
// untranslated messages are returned unchanged (Chinese source text).
func Translate(lang, msg string) string {
	if Normalize(lang) != English {
		return msg
	}
	if translated, ok := englishMessages[msg]; ok {
		return translated
	}
	return msg
}
