// "Nasıl çalışır", "Gizlilik ve KVKK" ve "Kullanım koşulları" metinleri (TR/EN).
// Metinler veri olarak tutulur ve textContent ile basılır; HTML üretilmez.
//
// Not: Gizlilik ve koşullar metinleri bir taslaktır; hukuki görüş değildir.

export type Block =
  | { h2: string }
  | { p: string }
  | { ul: string[] }
  | { table: [string, string][]; head: [string, string] }
  | { copy: string; label: string };

export type Page = { title: string; lead: string; updated: string; blocks: Block[] };

const UPDATED_TR = "Son güncelleme: 19 Ekim 2026";
const UPDATED_EN = "Last updated: 19 October 2026";

const how: Record<"tr" | "en", Page> = {
  tr: {
    title: "Nabız nasıl çalışır",
    lead: "Nabız, sitenin ziyaretçilerini tanımadan sayar. Bu sayfa neyin toplandığını, neyin toplanmadığını ve bir ziyaretin nasıl sayıldığını anlatıyor.",
    updated: UPDATED_TR,
    blocks: [
      { h2: "Bir ziyaret nasıl sayılır" },
      { p: "Sitene eklediğin betik 1 KB'tan küçük. Her sayfa açılışında sunucuya beş bilgi gönderir: olay adı, sayfanın adresi, gelinen sayfa, site adı ve ekran genişliği. Tarayıcıya hiçbir şey yazmaz: çerez, yerel depolama ya da parmak izi yok." },
      { p: "Sunucu, aynı kişinin gün içindeki ziyaretlerini bir arada sayabilmek için şu değeri hesaplar: SHA-256(günün tuzu + site + IP adresi + tarayıcı bilgisi). Tuz her gece yenilenir ve eskisi silinir. Böylece bugünkü değer yarın anlamsızlaşır; bir kişinin farklı günlerdeki ziyaretleri birbirine bağlanamaz, bu değerden de IP adresine geri dönülemez." },
      { p: "IP adresi sadece bu hesap ve ülkeyi bulmak için, istek sürerken bellekte kullanılır. Veritabanına ya da log'a hiç yazılmaz. Ülke, sunucudaki yerel bir veritabanından bulunur; IP adresi hiçbir üçüncü tarafa gönderilmez." },
      { h2: "Ne saklanır, ne saklanmaz" },
      {
        head: ["Saklanır", "Saklanmaz"],
        table: [
          ["Sayfa yolu (ör. /iletisim)", "IP adresi"],
          ["Gelinen sitenin sadece alan adı (ör. google.com)", "Gelinen sayfanın tam adresi ve arama kelimesi"],
          ["UTM kampanya bilgisi", "Çerez ya da tarayıcıda saklanan herhangi bir kimlik"],
          ["Ülke", "Şehir ya da konum"],
          ["Cihaz türü (ekran genişliğinden: mobil, tablet...)", "Ekran çözünürlüğü"],
          ["Tarayıcı ve işletim sistemi adı (sürüm yok)", "Tam tarayıcı bilgisi (User-Agent)"],
          ["Özel olay adları ve senin gönderdiğin özellikler", "E-postaya benzeyen özellik değerleri (sunucu reddeder)"],
          ["Günlük ziyaretçi değeri (ertesi gün anlamsız)", "Bir kişiyi günler arasında tanıyabilecek herhangi bir şey"],
        ],
      },
      { h2: "Sayılmayanlar" },
      { ul: [
        "Tarayıcısında \"Do Not Track\" açık olanlar: betik hiç istek göndermez.",
        "Botlar ve otomasyon araçları (arama motorları, bağlantı önizleyiciler, başsız tarayıcılar).",
        "localhost ve dosya olarak açılan sayfalar.",
        "Site sahibinin kendi ziyaretleri: tarayıcı konsolunda localStorage.nabiz_ignore = \"true\" yazman yeterli.",
      ] },
      { h2: "Veri nerede" },
      { p: "nabiz.miracdeprem.com'un sunucusu Almanya'da (Oracle Cloud, Frankfurt). IP adresi orada sadece bellekte işlenir; saklanan istatistikler kişisel veri içermez. Verinin Türkiye'de kalmasını istiyorsan Nabız'ı kendi sunucuna kurabilirsin: tek bir program dosyası ve SQLite veritabanı." },
      { h2: "Çerez onayı gerekir mi" },
      { p: "Nabız ziyaretçinin cihazına hiçbir şey yazmadığı ve ondan bir şey okumadığı için çerez onayı gerektiren bir işlem yapmaz. Yine de siteni ziyaret edenleri bilgilendirmen gerekir. Aşağıdaki metni sitenin gizlilik ya da aydınlatma metnine ekleyebilirsin; kendi durumuna göre düzenle." },
      { label: "Gizlilik metnine eklenecek bölüm", copy: "Ziyaret istatistikleri\n\nSitemizin hangi sayfalarının ne kadar ziyaret edildiğini anlamak için Nabız adlı çerezsiz bir analitik aracı kullanıyoruz. Nabız tarayıcınıza çerez ya da başka bir bilgi yazmaz ve sizi tanımlayan kişisel veri saklamaz. Ziyaretiniz sırasında IP adresiniz yalnızca ülkenizi belirlemek ve aynı gün içindeki ziyaretleri birlikte saymak için anlık olarak işlenir, hiçbir yere kaydedilmez. Saklanan bilgiler; ziyaret edilen sayfa, gelinen sitenin alan adı, ülke, cihaz türü, tarayıcı ve işletim sistemi adıdır ve sizinle ilişkilendirilemez. Tarayıcınızda \"Do Not Track\" ayarı açıksa ziyaretiniz hiç sayılmaz. Ayrıntılar: https://nabiz.miracdeprem.com/how-it-works" },
    ],
  },
  en: {
    title: "How Nabız works",
    lead: "Nabız counts your site's visitors without identifying them. This page explains what is collected, what isn't, and how a visit is counted.",
    updated: UPDATED_EN,
    blocks: [
      { h2: "How a visit is counted" },
      { p: "The script you add to your site is under 1 KB. On each page load it sends five things to the server: the event name, the page address, the referring page, the site name and the screen width. It writes nothing to the browser: no cookies, no local storage, no fingerprint." },
      { p: "To count the same person's visits within a day, the server computes SHA-256(the day's salt + site + IP address + browser string). The salt is replaced every night and the old one is deleted. Today's value becomes meaningless tomorrow; one person's visits on different days can't be linked, and the value can't be turned back into an IP address." },
      { p: "The IP address is used only in memory while the request runs, for this calculation and to find the country. It is never written to the database or logs. The country comes from a local database on the server; the IP address is never sent to a third party." },
      { h2: "What is stored and what isn't" },
      {
        head: ["Stored", "Not stored"],
        table: [
          ["Page path (e.g. /contact)", "IP address"],
          ["Only the domain of the referring site (e.g. google.com)", "The full referring address or search terms"],
          ["UTM campaign tags", "Cookies or any ID kept in the browser"],
          ["Country", "City or location"],
          ["Device type (from screen width: mobile, tablet...)", "Screen resolution"],
          ["Browser and operating system name (no version)", "The full browser string (User-Agent)"],
          ["Custom event names and the properties you send", "Property values that look like emails (the server rejects them)"],
          ["A daily visitor value (meaningless the next day)", "Anything that could recognize a person across days"],
        ],
      },
      { h2: "What isn't counted" },
      { ul: [
        "Visitors with \"Do Not Track\" on: the script sends nothing.",
        "Bots and automation (search engines, link previews, headless browsers).",
        "localhost and pages opened as files.",
        "The site owner's own visits: type localStorage.nabiz_ignore = \"true\" in the browser console.",
      ] },
      { h2: "Where the data is" },
      { p: "The server of nabiz.miracdeprem.com is in Germany (Oracle Cloud, Frankfurt). The IP address is processed there only in memory; the stored statistics contain no personal data. To keep data in your own country, install Nabız on your own server: one program file and an SQLite database." },
      { h2: "Do you need a cookie banner" },
      { p: "Nabız writes nothing to and reads nothing from the visitor's device, so it doesn't do anything that needs cookie consent. You should still tell your visitors. Add the text below to your site's privacy notice and adjust it to your situation." },
      { label: "Section for your privacy notice", copy: "Visit statistics\n\nTo understand how often our pages are visited, we use Nabız, a cookie-free analytics tool. Nabız writes no cookies or other data to your browser and stores no personal data that identifies you. During your visit your IP address is processed only for a moment, to find your country and to count visits on the same day together; it is not saved anywhere. What is stored is the page visited, the domain of the referring site, country, device type, and browser and operating system name, and it can't be linked to you. If \"Do Not Track\" is on in your browser, your visit isn't counted at all. Details: https://nabiz.miracdeprem.com/how-it-works" },
    ],
  },
};

const privacy: Record<"tr" | "en", Page> = {
  tr: {
    title: "Gizlilik ve KVKK aydınlatma metni",
    lead: "Bu metin, nabiz.miracdeprem.com'da hesap açanların kişisel verilerinin nasıl işlendiğini anlatır. Sitelerin ziyaretçileri hakkında kişisel veri saklanmaz; bunun için \"Nasıl çalışır\" sayfasına bak.",
    updated: UPDATED_TR,
    blocks: [
      { h2: "Veri sorumlusu" },
      { p: "Miraç Deprem. İletişim: https://www.miracdeprem.com/#contact" },
      { h2: "Hangi verileri işliyoruz" },
      { ul: [
        "E-posta adresin ve dil tercihin.",
        "Şifrenin Argon2id özeti (şifrenin kendisi saklanmaz, biz de göremeyiz).",
        "Giriş için oturum çerezi (HttpOnly; sadece giriş yaptığında, sadece bu sitede).",
        "Eklediğin sitelerin alan adları, hedeflerin, hunilerin ve takım üyeliklerin.",
        "Güvenlik için: istek sırasında IP adresin bellekte hız sınırı amacıyla kısa süre tutulur, kaydedilmez.",
      ] },
      { h2: "Neden işliyoruz ve hukuki sebep" },
      { p: "Hesabını açmak ve yönetmek, girişini güvende tutmak, doğrulama, şifre sıfırlama ve davet e-postalarını göndermek ve hizmetin kötüye kullanılmasını önlemek için. Hukuki sebep: sözleşmenin kurulması ve ifası (KVKK m. 5/2-c) ve hizmetin güvenliği için meşru menfaat (KVKK m. 5/2-f)." },
      { h2: "Kimlere aktarılır" },
      { p: "Verilerin, hizmetin çalıştığı Oracle Cloud sunucusunda (Frankfurt, Almanya) saklanır. E-postalar bir e-posta gönderim hizmeti üzerinden iletilir; ona sadece alıcı adresi ve e-postanın içeriği gider. Veriler satılmaz, reklam için kullanılmaz ve başka kimseyle paylaşılmaz." },
      { h2: "Ne kadar saklanır" },
      { p: "Hesabın açık olduğu sürece. Hesabını sildiğinde hesabın ve sahibi olduğun bütün siteler istatistikleriyle birlikte hemen silinir; gece alınan yedeklerden de en geç 7 gün içinde çıkar." },
      { h2: "Hakların" },
      { p: "KVKK'nın 11. maddesine göre şunları isteyebilirsin:" },
      { ul: [
        "Kişisel verilerinin işlenip işlenmediğini ve işleniyorsa buna ilişkin bilgiyi öğrenmek.",
        "İşlenme amacını ve amacına uygun kullanılıp kullanılmadığını öğrenmek.",
        "Aktarıldığı üçüncü kişileri bilmek.",
        "Eksik ya da yanlış işlenmişse düzeltilmesini, silinmesini ya da yok edilmesini istemek ve bunun aktarılan üçüncü kişilere bildirilmesini istemek.",
        "Otomatik sistemlerle analiz sonucu aleyhine bir sonuç çıkmasına itiraz etmek.",
        "Kanuna aykırı işleme sebebiyle zarara uğradıysan zararın giderilmesini istemek.",
      ] },
      { p: "Hesap ayarlarından e-postanı değiştirebilir ve hesabını bütün verileriyle kendin silebilirsin. Diğer talepler için yukarıdaki iletişim adresini kullan." },
      { h2: "Tarayıcıda tutulanlar" },
      { p: "Oturum çerezi dışında tarayıcında sadece dil ve tema tercihin saklanır (localStorage). Bunlar sunucuya gönderilmez." },
    ],
  },
  en: {
    title: "Privacy and data protection notice",
    lead: "This notice explains how the personal data of people who create an account on nabiz.miracdeprem.com is processed. No personal data is stored about sites' visitors; see \"How it works\" for that.",
    updated: UPDATED_EN,
    blocks: [
      { h2: "Data controller" },
      { p: "Miraç Deprem. Contact: https://www.miracdeprem.com/#contact" },
      { h2: "What we process" },
      { ul: [
        "Your email address and language preference.",
        "An Argon2id hash of your password (the password itself isn't stored and we can't see it).",
        "A session cookie for logging in (HttpOnly; only when you log in, only on this site).",
        "The domains of sites you add, your goals, funnels and team memberships.",
        "For security: your IP address is held briefly in memory during a request for rate limiting and isn't saved.",
      ] },
      { h2: "Why and on what legal basis" },
      { p: "To create and manage your account, keep your login safe, send verification, password reset and invite emails, and prevent abuse of the service. Legal basis: entering into and performing the contract (KVKK art. 5/2-c) and legitimate interest in keeping the service secure (KVKK art. 5/2-f)." },
      { h2: "Who receives it" },
      { p: "Your data is stored on the Oracle Cloud server the service runs on (Frankfurt, Germany). Emails go through an email delivery service, which receives only the recipient address and the email's content. Data isn't sold, used for ads or shared with anyone else." },
      { h2: "How long it's kept" },
      { p: "As long as your account exists. When you delete your account, it and every site you own are deleted at once with their statistics; they leave the nightly backups within 7 days." },
      { h2: "Your rights" },
      { p: "Under article 11 of Turkey's data protection law (KVKK) you can ask:" },
      { ul: [
        "Whether your personal data is processed, and information about it if so.",
        "The purpose of processing and whether it's used for that purpose.",
        "Which third parties it's transferred to.",
        "To correct it if it's incomplete or wrong, to delete or destroy it, and to have this passed on to those third parties.",
        "To object to an outcome against you that comes only from automated analysis.",
        "Compensation if you suffer damage from unlawful processing.",
      ] },
      { p: "You can change your email and delete your account with all its data yourself in account settings. For anything else, use the contact address above." },
      { h2: "What your browser keeps" },
      { p: "Besides the session cookie, your browser keeps only your language and theme choice (localStorage). These aren't sent to the server." },
    ],
  },
};

const terms: Record<"tr" | "en", Page> = {
  tr: {
    title: "Kullanım koşulları",
    lead: "nabiz.miracdeprem.com'u kullanarak bu koşulları kabul etmiş olursun. Kısa tuttuk.",
    updated: UPDATED_TR,
    blocks: [
      { h2: "Hizmet" },
      { p: "Nabız ücretsiz bir web analitiği hizmetidir ve \"olduğu gibi\" sunulur. Kesintisiz ya da hatasız çalışacağı garanti edilmez; verilerin için düzenli olarak CSV dışa aktarma kullanmanı öneririz." },
      { h2: "Sınırlar" },
      { p: "Her hesap en fazla 2 site ekleyebilir ve sahibi olduğu sitelerde ayda toplam 10.000 sayfa görüntülemesi sayılır. Sınır aşılınca veri silinmez; iki ay üst üste aşılırsa yeni site eklenemez. Daha fazlası için Nabız'ı kendi sunucuna sınırsız kurabilirsin." },
      { h2: "Senin sorumlulukların" },
      { ul: [
        "Takip betiğini sadece sahibi olduğun ya da yetkili olduğun sitelere ekle.",
        "Sitenin ziyaretçilerini bilgilendir (\"Nasıl çalışır\" sayfasında hazır bir metin var).",
        "Olay özelliklerinde (props) kişisel veri gönderme.",
        "Şifreni kimseyle paylaşma.",
      ] },
      { h2: "Yasak kullanım" },
      { ul: [
        "Sahte trafik üretmek ya da istatistikleri bilerek bozmak.",
        "Hizmeti zorlamak, güvenliğini aşmaya çalışmak ya da otomatik olarak hesap açmak.",
        "Hizmeti yasalara aykırı bir amaçla kullanmak.",
      ] },
      { p: "Bu kurallara aykırı kullanımda hesap uyarı yapılarak ya da gerekirse doğrudan kapatılabilir." },
      { h2: "Hesabın sonlandırılması" },
      { p: "Hesabını istediğin zaman ayarlardan bütün verileriyle silebilirsin. Hizmet sona erdirilecek olursa bunu en az 30 gün önceden e-postayla bildiririz; bu sürede verilerini dışa aktarabilirsin." },
      { h2: "Değişiklikler" },
      { p: "Koşullar değişebilir. Önemli bir değişiklik olursa yürürlüğe girmeden önce e-postayla haber veririz." },
      { h2: "Açık kaynak" },
      { p: "Nabız'ın kodu AGPL-3.0 lisansıyla açıktır: https://github.com/MrcDprm/nabiz-analytics" },
      { h2: "İletişim" },
      { p: "https://www.miracdeprem.com/#contact" },
    ],
  },
  en: {
    title: "Terms of use",
    lead: "By using nabiz.miracdeprem.com you accept these terms. We kept them short.",
    updated: UPDATED_EN,
    blocks: [
      { h2: "The service" },
      { p: "Nabız is a free web analytics service provided \"as is\". It isn't guaranteed to run without interruptions or errors; we suggest exporting your data as CSV regularly." },
      { h2: "Limits" },
      { p: "Each account can add up to 2 sites, and up to 10,000 page views a month are counted across the sites it owns. Going over doesn't delete data; two months in a row and you can't add new sites. For more, install Nabız on your own server with no limits." },
      { h2: "Your responsibilities" },
      { ul: [
        "Add the tracking script only to sites you own or are allowed to manage.",
        "Tell your site's visitors (\"How it works\" has a ready text).",
        "Don't send personal data in event properties (props).",
        "Don't share your password.",
      ] },
      { h2: "Not allowed" },
      { ul: [
        "Generating fake traffic or deliberately distorting statistics.",
        "Overloading the service, trying to get around its security, or creating accounts automatically.",
        "Using the service for anything unlawful.",
      ] },
      { p: "Accounts that break these rules can be closed, after a warning or directly if needed." },
      { h2: "Ending your account" },
      { p: "You can delete your account with all its data in settings at any time. If the service ever shuts down, we'll email you at least 30 days before so you can export your data." },
      { h2: "Changes" },
      { p: "These terms may change. We'll email you before an important change takes effect." },
      { h2: "Open source" },
      { p: "Nabız's code is open under the AGPL-3.0 license: https://github.com/MrcDprm/nabiz-analytics" },
      { h2: "Contact" },
      { p: "https://www.miracdeprem.com/#contact" },
    ],
  },
};

export const pages = { "how-it-works": how, privacy, terms };
export type PageKey = keyof typeof pages;
