const CHROME_VERSIONS = [
  '120.0.6099.109','120.0.6099.199','121.0.6167.85','121.0.6167.160',
  '122.0.6261.69','122.0.6261.112','123.0.6312.58','123.0.6312.105',
  '124.0.6367.60','124.0.6367.118','125.0.6422.60','125.0.6422.113',
];
const PLATFORMS = {
  windows: v => `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/${v} Safari/537.36`,
  mac: v => `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/${v} Safari/537.36`,
  linux: v => `Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/${v} Safari/537.36`,
};
const COUNTRY_PROFILES = {
  US:['windows','en-US','en-US,en;q=0.9'],GB:['windows','en-GB','en-GB,en;q=0.9'],
  CA:['windows','en-CA','en-CA,en;q=0.9'],AU:['windows','en-AU','en-AU,en;q=0.9'],
  DE:['windows','de-DE','de-DE,de;q=0.9,en;q=0.8'],FR:['windows','fr-FR','fr-FR,fr;q=0.9,en;q=0.8'],
  ES:['windows','es-ES','es-ES,es;q=0.9,en;q=0.8'],IT:['windows','it-IT','it-IT,it;q=0.9,en;q=0.8'],
  PT:['windows','pt-PT','pt-PT,pt;q=0.9,en;q=0.8'],BR:['windows','pt-BR','pt-BR,pt;q=0.9,en;q=0.8'],
  NL:['windows','nl-NL','nl-NL,nl;q=0.9,en;q=0.8'],PL:['windows','pl-PL','pl-PL,pl;q=0.9,en;q=0.8'],
  RU:['windows','ru-RU','ru-RU,ru;q=0.9,en;q=0.8'],UA:['windows','uk-UA','uk-UA,uk;q=0.9,en;q=0.8'],
  JP:['mac','ja-JP','ja-JP,ja;q=0.9,en;q=0.8'],KR:['windows','ko-KR','ko-KR,ko;q=0.9,en;q=0.8'],
  CN:['windows','zh-CN','zh-CN,zh;q=0.9,en;q=0.8'],TW:['windows','zh-TW','zh-TW,zh;q=0.9,en;q=0.8'],
  IN:['windows','en-IN','en-IN,en;q=0.9,hi;q=0.8'],ID:['windows','id-ID','id-ID,id;q=0.9,en;q=0.8'],
  TH:['windows','th-TH','th-TH,th;q=0.9,en;q=0.8'],VN:['windows','vi-VN','vi-VN,vi;q=0.9,en;q=0.8'],
  TR:['windows','tr-TR','tr-TR,tr;q=0.9,en;q=0.8'],MX:['windows','es-MX','es-MX,es;q=0.9,en;q=0.8'],
  AR:['windows','es-AR','es-AR,es;q=0.9,en;q=0.8'],IL:['windows','he-IL','he-IL,he;q=0.9,en;q=0.8'],
  SE:['windows','sv-SE','sv-SE,sv;q=0.9,en;q=0.8'],NO:['windows','nb-NO','nb-NO,nb;q=0.9,en;q=0.8'],
};
function randomUA(platform='windows') {
  const v = CHROME_VERSIONS[Math.floor(Math.random()*CHROME_VERSIONS.length)];
  return (PLATFORMS[platform]||PLATFORMS.windows)(v);
}
function getHeaders(country='US') {
  const [platform,lang,acceptLang] = COUNTRY_PROFILES[country.toUpperCase()]||COUNTRY_PROFILES.US;
  return {
    'User-Agent': randomUA(platform),
    'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8',
    'Accept-Language': acceptLang,
    'Accept-Encoding': 'gzip, deflate, br',
    'Connection': 'keep-alive',
    'Upgrade-Insecure-Requests': '1',
    'Sec-Fetch-Dest': 'document',
    'Sec-Fetch-Mode': 'navigate',
    'Sec-Fetch-Site': 'none',
    'Sec-Fetch-User': '?1',
    'Sec-Ch-Ua-Platform': platform==='windows'?'"Windows"':platform==='mac'?'"macOS"':'"Linux"',
  };
}
module.exports = { getHeaders, randomUA, COUNTRY_PROFILES, CHROME_VERSIONS };
