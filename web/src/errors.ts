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
