// Sunucu, kullanıcının düzeltebileceği hataları Türkçe mesajla döndürür.
// Panel İngilizceyken bu mesajların karşılığı burada.
const en: Record<string, string> = {
  "şifre en az 10 karakter olmalı": "Password must be at least 10 characters.",
  "şifre en fazla 128 karakter olabilir": "Password can be at most 128 characters.",
  "şifrede en az bir harf ve bir rakam olmalı": "Password needs at least one letter and one digit.",
  "bu şifre çok yaygın, başka bir şifre seçin": "This password is too common; pick another one.",
  "şifre e-posta adresinizi içermemeli": "Password must not contain your email address.",
  "geçerli bir e-posta adresi yazın": "Type a valid email address.",
  "bağlantı geçersiz ya da süresi dolmuş": "This link is invalid or has expired.",
  "bu zaten hesabının e-posta adresi": "That's already your account's email.",
  "bu e-posta ile bir hesap zaten var": "There's already an account with this email.",
  "mevcut şifre yanlış": "Current password is wrong.",
  "bu site zaten ekli": "This site is already added.",
  "geçerli bir alan adı yazın (ör. example.com)": "Type a valid domain (e.g. example.com).",
  "site eklemeden önce e-posta adresinizi doğrulayın": "Verify your email before adding a site.",
  "bu hedef zaten var": "This goal already exists.",
  "bir sitede en fazla 20 hedef olabilir": "A site can have at most 20 goals.",
  "bir sitede en fazla 10 huni olabilir": "A site can have at most 10 funnels.",
  "bu işlem için yetkiniz yok": "You don't have permission for this.",
  "rol yönetici ya da izleyici olmalı": "Role must be admin or viewer.",
  "bu kişi zaten sitenin üyesi": "This person is already a member.",
  "bu kişi sitenin üyesi değil": "This person isn't a member.",
  "sahibin rolü değiştirilemez; önce sahipliği devret": "The owner's role can't change; transfer ownership first.",
  "davet geçersiz ya da süresi dolmuş": "This invite is invalid or has expired.",
  "bu davet başka bir e-posta adresine gönderildi": "This invite was sent to another email address.",
  "sahiplik zaten sende": "You already own this site.",
  "paketinin site sınırına ulaştın; daha fazla site için paketini yükselt": "You've reached your plan's site limit; upgrade for more sites.",
  "paketinin takım sınırına ulaştın; daha fazla üye için paketini yükselt": "You've reached your plan's team limit; upgrade for more members.",
  "iki aydır görüntüleme sınırını aşıyorsun; yeni site eklemek için paketini yükselt": "You've been over your page view limit for two months; upgrade to add new sites.",
  "geçersiz paket ya da dönem": "Invalid plan or billing period.",
  "aktif bir aboneliğin yok": "You don't have an active subscription.",
  "zaten aktif bir aboneliğin var; paket değiştirmeyi kullan": "You already have an active subscription; switch plans instead.",
  "dönem değiştirmek için aboneliği iptal edip yeniden başlat": "To change the billing period, cancel and start again.",
  "devam etmek için sözleşmeleri onaylaman gerekiyor": "Accept the contracts to continue.",
  "fatura bilgilerini eksiksiz doldur": "Fill in all billing details.",
  "telefon numarası geçersiz (ör. +905551112233)": "Invalid phone number (e.g. +905551112233).",
  "TC kimlik numarası geçersiz": "Invalid Turkish ID number.",
  "ödemeden önce e-posta adresinizi doğrulayın": "Verify your email before paying.",
  "ödeme sağlayıcısı isteği reddetti; bilgilerini kontrol edip tekrar dene": "The payment provider rejected the request; check your details and try again.",
  "ücretsiz sürümün site sınırına ulaştın; sınırsız kullanım için Nabız'ı kendi sunucuna kurabilirsin": "You've reached the free version's site limit; install Nabız on your own server for unlimited use.",
  "ücretsiz sürümde takım daveti yok; takım için Nabız'ı kendi sunucuna kurabilirsin": "The free version has no team invites; install Nabız on your own server for a team.",
  "iki aydır görüntüleme sınırını aşıyorsun; yeni site eklemek için Nabız'ı kendi sunucuna kurabilirsin": "You've been over the page view limit for two months; install Nabız on your own server to add new sites.",
  "bu sunucuda ödeme kapalı": "Payments are off on this server.",
};

export function serverMessage(msg: string, lang: "tr" | "en"): string | null {
  if (!msg) return null;
  if (lang === "tr") return msg.charAt(0).toUpperCase() + msg.slice(1);
  if (en[msg]) return en[msg];
  // Hedef/huni doğrulama mesajları uzun; İngilizcede kısa karşılık.
  if (msg.startsWith("hedef geçersiz")) return "Invalid goal: a page starts with / (max 512 characters), an event name is at most 64 characters.";
  if (msg.startsWith("huni geçersiz")) return "Invalid funnel: name 1-64 characters, 2-8 steps; each a page (starting with /) or an event name.";
  return null;
}
