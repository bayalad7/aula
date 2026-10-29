; Instalador de la consola del profesor (Aula Consola) — Inno Setup 6.
;
; Compilar (lo hace el workflow de release):
;   ISCC /DAppVersion=1.2.3 /DNumericVersion=1.2.3 /DSourceDir=..\..\dist /O..\..\dist aula-consola.iss
;
; El asistente pide (docs/uso-hito1.md):
;   1. El canal predeterminado: la consola arranca en él y muestra a sus alumnos.
;   2. La clave del centro: crearla (primera consola del centro), importarla
;      (aula-centro.key de otra consola) o mantener la que ya tiene el equipo.
;
; Silencioso:
;   Aula-Consola-Setup-X.Y.Z.exe /VERYSILENT /SUPPRESSMSGBOXES /NORESTART /CANAL=1
;     [/CENTRO=crear|mantener|<ruta a aula-centro.key>] [/SALIDA=<carpeta para aula-centro.pub/.key>]
;   Sin /CENTRO: mantiene la clave si existe; si no, la crea y deja aula-centro.pub y
;   aula-centro.key en /SALIDA (por defecto, junto al instalador).

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

[Dirs]
Name: "{commonappdata}\Aula\consola"

[Icons]
Name: "{autoprograms}\Aula Consola"; Filename: "{app}\aula-consola.exe"
Name: "{autodesktop}\Aula Consola"; Filename: "{app}\aula-consola.exe"; Tasks: desktopicon

[Run]
; Machine settings (default channel): readable by every teacher, changed only by administrators.
Filename: "{sys}\icacls.exe"; \
  Parameters: """{commonappdata}\Aula\consola"" /inheritance:r /grant:r *S-1-5-32-544:(OI)(CI)F *S-1-5-18:(OI)(CI)F *S-1-5-32-545:(OI)(CI)RX"; \
  Flags: runhidden
; Agents connect in (TCP 47830, TLS) and mDNS questions arrive in (UDP 5353).
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall delete rule name=""Aula Consola"""; Flags: runhidden
Filename: "{sys}\netsh.exe"; \
  Parameters: "advfirewall firewall add rule name=""Aula Consola"" dir=in action=allow program=""{app}\aula-consola.exe"" profile=private,domain"; \
  Flags: runhidden; StatusMsg: "Configurando el firewall…"
Filename: "{app}\aula-consola.exe"; Description: "Abrir Aula Consola"; \
  Flags: nowait postinstall skipifsilent runasoriginaluser

[UninstallRun]
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall delete rule name=""Aula Consola"""; \
  Flags: runhidden; RunOnceId: "DeleteFirewallRule"

[UninstallDelete]
; The site key stays (other consoles and every student depend on it); only settings go.
Type: files; Name: "{commonappdata}\Aula\consola\config.json"
Type: dirifempty; Name: "{commonappdata}\Aula\consola"

[Code]
var
  ChannelPage: TInputQueryWizardPage;
  KeyPage: TInputOptionWizardPage;
  ImportPage: TInputFileWizardPage;
  OutputPage: TInputDirWizardPage;

function SiteKeyPath: String;
begin
  Result := ExpandConstant('{commonappdata}\Aula\centro\clave-centro.pem');
end;

function SiteKeyExists: Boolean;
begin
  Result := FileExists(SiteKeyPath);
end;

{ Default channel from /CANAL=, the previous installation, or 1. }
function InitialChannel: String;
var
  Data: AnsiString;
  P, Q: Integer;
begin
  Result := ExpandConstant('{param:CANAL|}');
  if Result <> '' then Exit;
  Result := '1';
  if LoadStringFromFile(ExpandConstant('{commonappdata}\Aula\consola\config.json'), Data) then
  begin
    P := Pos('"default_channel":', String(Data));
    if P > 0 then
    begin
      Q := P + Length('"default_channel":');
      Result := '';
      while (Q <= Length(Data)) and ((Data[Q] = ' ') or ((Data[Q] >= '0') and (Data[Q] <= '9'))) do
      begin
        if Data[Q] <> ' ' then Result := Result + Data[Q];
        Q := Q + 1;
      end;
      if Result = '' then Result := '1';
    end;
  end;
end;

function ValidChannel(S: String): Boolean;
var
  N: Integer;
begin
  N := StrToIntDef(Trim(S), 0);
  Result := (N >= 1) and (N <= 16000);
end;

procedure InitializeWizard;
begin
  ChannelPage := CreateInputQueryPage(wpSelectDir,
    'Canal predeterminado',
    'La consola arranca en este canal y muestra a sus alumnos.',
    'Escribe el número del canal de la sala de este equipo (del 1 al 16000). Por ejemplo, 1 para el ' +
    'centro de cómputo principal y 2 para el alterno. El profesor puede cambiar de canal en la consola.');
  ChannelPage.Add('Canal:', False);
  ChannelPage.Values[0] := InitialChannel;

  KeyPage := CreateInputOptionPage(ChannelPage.ID,
    'Clave del centro',
    'Los alumnos solo obedecen a las consolas que tienen la clave de su centro.',
    'Hay una sola clave por centro de cómputo. Elige qué hacer en este equipo:', True, False);
  KeyPage.Add('Crear una clave nueva (es la primera consola de este centro)');
  KeyPage.Add('Importar la clave del centro (archivo aula-centro.key de otra consola)');
  KeyPage.Add('Mantener la clave del centro que ya tiene este equipo');
  if SiteKeyExists then KeyPage.SelectedValueIndex := 2 else KeyPage.SelectedValueIndex := 0;

  ImportPage := CreateInputFilePage(KeyPage.ID,
    'Importar la clave del centro',
    'Selecciona el archivo aula-centro.key exportado de otra consola del centro.',
    'Después de instalar, borra ese archivo de la memoria USB o carpeta compartida: es secreto.');
  ImportPage.Add('Archivo aula-centro.key:', 'Clave del centro (*.key)|*.key|Todos los archivos|*.*', '.key');

  OutputPage := CreateInputDirPage(KeyPage.ID,
    'Archivos para instalar el resto del centro',
    'Dónde guardar aula-centro.pub (para los alumnos) y aula-centro.key (para otras consolas).',
    'aula-centro.pub va junto a Aula-Alumno-Setup en cada equipo de alumno. aula-centro.key es SECRETO: ' +
    'úsalo solo para instalar otras consolas y guárdalo en un lugar seguro.', False, '');
  OutputPage.Add('');
  OutputPage.Values[0] := ExpandConstant('{param:SALIDA|{userdocs}\Aula}');
end;

function ShouldSkipPage(PageID: Integer): Boolean;
begin
  Result := False;
  if PageID = ImportPage.ID then Result := KeyPage.SelectedValueIndex <> 1;
  if PageID = OutputPage.ID then Result := KeyPage.SelectedValueIndex <> 0;
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if (CurPageID = ChannelPage.ID) and not ValidChannel(ChannelPage.Values[0]) then
  begin
    MsgBox('El canal debe ser un número del 1 al 16000.', mbError, MB_OK);
    Result := False;
  end;
  if (CurPageID = KeyPage.ID) and (KeyPage.SelectedValueIndex = 2) and not SiteKeyExists then
  begin
    MsgBox('Este equipo todavía no tiene clave del centro: crea una o impórtala.', mbError, MB_OK);
    Result := False;
  end;
  if (CurPageID = ImportPage.ID) and not FileExists(ImportPage.Values[0]) then
  begin
    MsgBox('Selecciona el archivo aula-centro.key.', mbError, MB_OK);
    Result := False;
  end;
end;

{ Runs aula-consola.exe with a CLI command; returns True on exit code 0. }
function RunConsole(Params: String): Boolean;
var
  Code: Integer;
begin
  Result := Exec(ExpandConstant('{app}\aula-consola.exe'), Params, '', SW_HIDE, ewWaitUntilTerminated, Code) and (Code = 0);
  Log(Format('aula-consola %s -> %d', [Params, Code]));
end;

procedure Fail(Msg: String);
begin
  Log('ERROR: ' + Msg);
  if not WizardSilent then MsgBox(Msg, mbError, MB_OK);
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  Channel, Mode, OutDir: String;
begin
  if CurStep <> ssPostInstall then Exit;

  { 1. Default channel. }
  Channel := IntToStr(StrToIntDef(Trim(ChannelPage.Values[0]), 1));
  if not ValidChannel(Channel) then Channel := '1';
  if not SaveStringToFile(ExpandConstant('{commonappdata}\Aula\consola\config.json'),
      '{' + #13#10 + '  "default_channel": ' + Channel + #13#10 + '}' + #13#10, False) then
    Fail('No se pudo guardar el canal predeterminado.');

  { 2. Site key. In silent mode /CENTRO decides; by default keep it or create it. }
  Mode := ExpandConstant('{param:CENTRO|}');
  if not WizardSilent then
    case KeyPage.SelectedValueIndex of
      0: Mode := 'crear';
      1: Mode := ImportPage.Values[0];
      2: Mode := 'mantener';
    end
  else if Mode = '' then
    if SiteKeyExists then Mode := 'mantener' else Mode := 'crear';

  if Mode = 'mantener' then Exit;
  if Mode = 'crear' then
  begin
    if SiteKeyExists then
    begin
      Fail('Este equipo ya tiene clave del centro; se mantiene. Para reemplazarla usa "aula-consola centro crear -force".');
      Exit;
    end;
    if not RunConsole('centro crear') then
    begin
      Fail('No se pudo crear la clave del centro (ver el registro de instalación).');
      Exit;
    end;
    OutDir := OutputPage.Values[0];
    if WizardSilent then OutDir := ExpandConstant('{param:SALIDA|{src}}');
    ForceDirectories(OutDir);
    if not RunConsole('centro publica -o "' + AddBackslash(OutDir) + 'aula-centro.pub"') or
       not RunConsole('centro exportar "' + AddBackslash(OutDir) + 'aula-centro.key"') then
      Fail('La clave se creó, pero no se pudieron guardar aula-centro.pub y aula-centro.key en ' + OutDir +
        '. Hazlo con "aula-consola centro publica" y "aula-consola centro exportar".')
    else if not WizardSilent then
      MsgBox('Clave del centro creada.' + #13#10 + #13#10 +
        'En ' + OutDir + ' quedaron:' + #13#10 +
        '  • aula-centro.pub — cópialo junto a Aula-Alumno-Setup para instalar los alumnos.' + #13#10 +
        '  • aula-centro.key — SECRETO: solo para instalar otras consolas del centro.', mbInformation, MB_OK);
  end
  else if not RunConsole('centro importar "' + Mode + '"') then
    Fail('No se pudo importar la clave del centro desde ' + Mode + '.');
end;
