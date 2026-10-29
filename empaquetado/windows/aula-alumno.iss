; Instalador del agente del alumno (Aula Cliente) — Inno Setup 6.
;
; Compilar (lo hace el workflow de release):
;   ISCC /DAppVersion=1.2.3 /DNumericVersion=1.2.3 /DSourceDir=..\..\dist /O..\..\dist aula-alumno.iss
;
; El asistente pide el CANAL de esta computadora (p. ej. 1 = centro de cómputo principal,
; 2 = alterno) y la clave pública del centro, que toma de aula-centro.pub junto al setup.
; Solo un administrador puede cambiarlos después (reinstalando).
;
; Despliegue silencioso (GPO, etapa 15):
;   Aula-Alumno-Setup-X.Y.Z.exe /VERYSILENT /SUPPRESSMSGBOXES /NORESTART /CANAL=2
;     [/CLAVECENTRO=<ruta a aula-centro.pub | aula-ed25519:…>] [/DIRECTO=IP:47830]
;     [/MERGETASKS="redpublica"]  (abre también el perfil Público del firewall, D-012)
;
; Formato de la configuración que escribe: docs/configuracion.md.

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
AppId={{9B2E6E0A-5C1F-4F43-9D1E-6A1B2C3D4E51}
AppName=Aula Cliente
AppVersion={#AppVersion}
AppVerName=Aula Cliente {#AppVersion}
AppPublisher=Aula
VersionInfoVersion={#NumericVersion}
VersionInfoProductVersion={#NumericVersion}
VersionInfoDescription=Instalador de Aula Cliente (agente del alumno)
DefaultDirName={autopf}\Aula\Cliente
DisableProgramGroupPage=yes
DisableDirPage=auto
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputBaseFilename=Aula-Alumno-Setup-{#AppVersion}
SetupIconFile=..\..\apps\student\build\windows\icon.ico
UninstallDisplayIcon={app}\aula-cliente.exe
UninstallDisplayName=Aula Cliente
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
; Close a running agent when upgrading; do not relaunch it as administrator.
CloseApplications=yes
RestartApplications=no
SetupLogging=yes

[Languages]
Name: "spanish"; MessagesFile: "compiler:Languages\Spanish.isl"

[Tasks]
; Off by default: the agent should not answer mDNS on untrusted networks (D-012).
Name: "redpublica"; Description: "Permitir también en redes Públicas (solo si la red del laboratorio no puede ser Privada)"; \
  GroupDescription: "Firewall:"; Flags: unchecked

[Files]
Source: "{#SourceDir}\aula-cliente.exe"; DestDir: "{app}"; Flags: ignoreversion

[Dirs]
Name: "{commonappdata}\Aula\alumno"

[Icons]
Name: "{autoprograms}\Aula Cliente"; Filename: "{app}\aula-cliente.exe"

[Registry]
; Starts with every user session (the system service arrives in phase 2).
Root: HKLM; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; \
  ValueName: "AulaCliente"; ValueData: """{app}\aula-cliente.exe"""; Flags: uninsdeletevalue

[Run]
; Only administrators may change the channel or the key: otherwise a student
; could move the computer to another room or point it at another console.
Filename: "{sys}\icacls.exe"; \
  Parameters: """{commonappdata}\Aula\alumno"" /inheritance:r /grant:r *S-1-5-32-544:(OI)(CI)F *S-1-5-18:(OI)(CI)F *S-1-5-32-545:(OI)(CI)RX"; \
  Flags: runhidden; StatusMsg: "Protegiendo la configuración…"
; mDNS answers arrive as inbound UDP: allow them for this program on private and domain networks.
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall delete rule name=""Aula Cliente (mDNS)"""; Flags: runhidden
Filename: "{sys}\netsh.exe"; \
  Parameters: "advfirewall firewall add rule name=""Aula Cliente (mDNS)"" dir=in action=allow program=""{app}\aula-cliente.exe"" protocol=UDP profile=private,domain"; \
  Flags: runhidden; StatusMsg: "Configurando el firewall…"; Tasks: not redpublica
Filename: "{sys}\netsh.exe"; \
  Parameters: "advfirewall firewall add rule name=""Aula Cliente (mDNS)"" dir=in action=allow program=""{app}\aula-cliente.exe"" protocol=UDP profile=private,domain,public"; \
  Flags: runhidden; StatusMsg: "Configurando el firewall…"; Tasks: redpublica
Filename: "{app}\aula-cliente.exe"; Description: "Abrir Aula Cliente"; \
  Flags: nowait postinstall skipifsilent runasoriginaluser

[UninstallRun]
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall delete rule name=""Aula Cliente (mDNS)"""; \
  Flags: runhidden; RunOnceId: "DeleteFirewallRule"

[UninstallDelete]
Type: files; Name: "{commonappdata}\Aula\alumno\config.json"
Type: dirifempty; Name: "{commonappdata}\Aula\alumno"
Type: dirifempty; Name: "{commonappdata}\Aula"

[Code]
var
  SettingsPage: TInputQueryWizardPage;

function ConfigPath: String;
begin
  Result := ExpandConstant('{commonappdata}\Aula\alumno\config.json');
end;

{ Value of "name": "value" in the existing config.json, or ''. }
function ExistingValue(Name: String): String;
var
  Data: AnsiString;
  S: String;
  P: Integer;
begin
  Result := '';
  if not LoadStringFromFile(ConfigPath, Data) then Exit;
  S := String(Data);
  P := Pos('"' + Name + '": "', S);
  if P = 0 then Exit;
  S := Copy(S, P + Length(Name) + 5, Length(S));
  P := Pos('"', S);
  if P > 0 then Result := Copy(S, 1, P - 1);
end;

{ /CLAVECENTRO= (a key or a file), aula-centro.pub next to the setup, or the existing key. }
function InitialKey: String;
var
  Data: AnsiString;
  Src: String;
begin
  Src := ExpandConstant('{param:CLAVECENTRO|}');
  if Pos('aula-ed25519:', Src) = 1 then
  begin
    Result := Src;
    Exit;
  end;
  if Src = '' then Src := ExpandConstant('{src}\aula-centro.pub');
  if FileExists(Src) and LoadStringFromFile(Src, Data) then
    Result := Trim(String(Data))
  else
    Result := ExistingValue('console_public_key');
end;

function InitialChannel: String;
begin
  Result := ExpandConstant('{param:CANAL|}');
  if Result = '' then Result := ExistingValue('channel');
  if Result = '' then Result := '1';
end;

function ValidChannel(S: String): Boolean;
var
  N: Integer;
begin
  N := StrToIntDef(Trim(S), 0);
  Result := (N >= 1) and (N <= 16000);
end;

{ "aula-ed25519:" followed by 44 Base64 characters (32 bytes). }
function ValidKey(S: String): Boolean;
begin
  S := Trim(S);
  Result := (Pos('aula-ed25519:', S) = 1) and (Length(S) = Length('aula-ed25519:') + 44);
end;

procedure InitializeWizard;
begin
  SettingsPage := CreateInputQueryPage(wpSelectDir,
    'Canal y clave del centro',
    'A qué sala pertenece esta computadora.',
    'Canal: número de la sala (del 1 al 16000), por ejemplo 1 para el centro de cómputo principal y 2 ' +
    'para el alterno. La computadora solo se conecta con las consolas de ese canal.' + #13#10 + #13#10 +
    'Clave del centro: se lee de aula-centro.pub junto a este instalador. Solo un administrador puede ' +
    'cambiar estos datos después, volviendo a instalar.');
  SettingsPage.Add('Canal de esta computadora:', False);
  SettingsPage.Add('Clave pública del centro (aula-ed25519:…):', False);
  SettingsPage.Values[0] := InitialChannel;
  SettingsPage.Values[1] := InitialKey;
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if CurPageID <> SettingsPage.ID then Exit;
  if not ValidChannel(SettingsPage.Values[0]) then
  begin
    MsgBox('El canal debe ser un número del 1 al 16000.', mbError, MB_OK);
    Result := False;
  end
  else if not ValidKey(SettingsPage.Values[1]) then
  begin
    MsgBox('Falta la clave pública del centro. Pon aula-centro.pub junto a este instalador o pega la ' +
      'clave (aula-ed25519:…) que muestra "aula-consola centro publica" en la consola.', mbError, MB_OK);
    Result := False;
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  Channel, Key, Direct, Json: String;
begin
  if CurStep <> ssPostInstall then Exit;
  Channel := Trim(SettingsPage.Values[0]);
  Key := Trim(SettingsPage.Values[1]);
  if not ValidChannel(Channel) or not ValidKey(Key) then
  begin
    { Silent install with missing /CANAL or key: the agent will show "sin configurar". }
    Log('ERROR: canal o clave del centro inválidos; no se escribe la configuración');
    if not WizardSilent then
      MsgBox('Aula Cliente quedó instalado pero sin configurar (canal o clave inválidos).', mbError, MB_OK);
    Exit;
  end;
  Channel := IntToStr(StrToInt(Channel));
  Json := '{' + #13#10 + '  "channel": "' + Channel + '",' + #13#10 + '  "console_public_key": "' + Key + '"';
  Direct := ExpandConstant('{param:DIRECTO|}');
  if Direct = '' then Direct := ExistingValue('direct');
  if Direct <> '' then Json := Json + ',' + #13#10 + '  "direct": "' + Direct + '"';
  Json := Json + #13#10 + '}' + #13#10;
  if SaveStringToFile(ConfigPath, Json, False) then
    Log('Configuración escrita: canal ' + Channel)
  else
  begin
    Log('ERROR: no se pudo escribir ' + ConfigPath);
    if not WizardSilent then MsgBox('No se pudo guardar la configuración en ' + ConfigPath, mbError, MB_OK);
  end;
end;
