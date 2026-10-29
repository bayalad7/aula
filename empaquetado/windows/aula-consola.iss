; Instalador de la consola del profesor (Aula Consola) — Inno Setup 6.
;
; Compilar (lo hace el workflow de release):
;   ISCC /DAppVersion=1.2.3 /DNumericVersion=1.2.3 /DSourceDir=..\..\dist /O..\..\dist aula-consola.iss
;
; La consola también se publica como ejecutable portable (aula-consola-<versión>.exe); este
; instalador además agrega accesos directos y la regla de firewall, para que Windows no
; pregunte en clase.

#ifndef AppVersion
  #define AppVersion "0.0.0-dev"
#endif
#ifndef NumericVersion
  #define NumericVersion "0.0.0"
#endif
#ifndef SourceDir
  #define SourceDir "..\..\dist"
#endif

[Setup]
AppId={{4C7A1F3E-2B8D-4E6A-9F05-7D3C2B1A0E92}
AppName=Aula Consola
AppVersion={#AppVersion}
AppVerName=Aula Consola {#AppVersion}
AppPublisher=Aula
VersionInfoVersion={#NumericVersion}
VersionInfoProductVersion={#NumericVersion}
VersionInfoDescription=Instalador de Aula Consola (profesor)
DefaultDirName={autopf}\Aula\Consola
DisableProgramGroupPage=yes
DisableDirPage=auto
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputBaseFilename=Aula-Consola-Setup-{#AppVersion}
SetupIconFile=..\..\apps\teacher\build\windows\icon.ico
UninstallDisplayIcon={app}\aula-consola.exe
UninstallDisplayName=Aula Consola
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
CloseApplications=yes
RestartApplications=no
SetupLogging=yes

[Languages]
Name: "spanish"; MessagesFile: "compiler:Languages\Spanish.isl"

[Tasks]
Name: "desktopicon"; Description: "Crear un acceso directo en el escritorio"; Flags: unchecked

[Files]
Source: "{#SourceDir}\aula-consola.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\Aula Consola"; Filename: "{app}\aula-consola.exe"
Name: "{autodesktop}\Aula Consola"; Filename: "{app}\aula-consola.exe"; Tasks: desktopicon

[Run]
; Agents connect in (TCP 47830, TLS) and mDNS questions arrive in (UDP 5353):
; allow this program on private and domain networks. The key stays in each
; teacher's %AppData%, never in the installation folder.
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall delete rule name=""Aula Consola"""; Flags: runhidden
Filename: "{sys}\netsh.exe"; \
  Parameters: "advfirewall firewall add rule name=""Aula Consola"" dir=in action=allow program=""{app}\aula-consola.exe"" profile=private,domain"; \
  Flags: runhidden; StatusMsg: "Configurando el firewall…"
Filename: "{app}\aula-consola.exe"; Description: "Abrir Aula Consola"; \
  Flags: nowait postinstall skipifsilent runasoriginaluser

[UninstallRun]
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall delete rule name=""Aula Consola"""; \
  Flags: runhidden; RunOnceId: "DeleteFirewallRule"
