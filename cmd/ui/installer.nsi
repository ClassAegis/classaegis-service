; OneClassMesh agent installer (壹课管家).
; Root stays clean: each component in its own folder.
; Build: makensis -DVER=x.y.z installer.nsi   (from cmd/ui)

Unicode true
!include "MUI2.nsh"

!define APP_NAME "壹课管家"
!ifndef VER
!define VER "0.0.0-dev"
!endif
!define REG_UNINST "Software\Microsoft\Windows\CurrentVersion\Uninstall\OneClassMesh"
!define REG_RUN "Software\Microsoft\Windows\CurrentVersion\Run"

Name "${APP_NAME}"
OutFile "release\OneClassMesh-Agent-${VER}-setup.exe"
InstallDir "$PROGRAMFILES64\OneClassMesh"
; upgrade in place keeps the user's chosen directory
InstallDirRegKey HKLM "${REG_UNINST}" "InstallLocation"
RequestExecutionLevel admin
SetCompressor /SOLID lzma

!define MUI_ABORTWARNING
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"

Section "Install"
  SetShellVarContext all
  SetOutPath "$INSTDIR"

  ; clean root: one folder per component
  File /r "installer-stage\壹课管家"
  File /r "installer-stage\bin"
  File /r "installer-stage\conf"

  ; desktop + start menu shortcuts -> launcher guards core/worker and opens UI
  CreateShortcut "$DESKTOP\壹课管家.lnk" "$INSTDIR\bin\launcher.exe" "-with-ui" "$INSTDIR\壹课管家\壹课管家.exe"
  CreateDirectory "$SMPROGRAMS\OneClassMesh"
  CreateShortcut "$SMPROGRAMS\OneClassMesh\壹课管家.lnk" "$INSTDIR\bin\launcher.exe" "-with-ui" "$INSTDIR\壹课管家\壹课管家.exe"

  ; registry writes go to the NATIVE 64-bit view (32-bit installer would
  ; otherwise land everything in WOW6432Node)
  SetRegView 64

  ; boot autostart at user logon
  WriteRegStr HKLM "${REG_RUN}" "OneClassMesh" '"$INSTDIR\bin\launcher.exe" -with-ui'

  ; Apps & features entry
  WriteRegStr HKLM "${REG_UNINST}" "DisplayName" "${APP_NAME} (OneClassMesh Agent)"
  WriteRegStr HKLM "${REG_UNINST}" "DisplayVersion" "${VER}"
  WriteRegStr HKLM "${REG_UNINST}" "Publisher" "OneClassMesh"
  WriteRegStr HKLM "${REG_UNINST}" "DisplayIcon" "$INSTDIR\壹课管家\壹课管家.exe"
  WriteRegStr HKLM "${REG_UNINST}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKLM "${REG_UNINST}" "UninstallString" '"$INSTDIR\Uninstall 壹课管家.exe"'
  WriteRegDWORD HKLM "${REG_UNINST}" "NoModify" 1
  WriteRegDWORD HKLM "${REG_UNINST}" "NoRepair" 1
  SetRegView lastused

  WriteUninstaller "$INSTDIR\Uninstall 壹课管家.exe"
SectionEnd

Section "Uninstall"
  SetShellVarContext all
  nsExec::Exec 'taskkill /f /im 壹课管家.exe'
  nsExec::Exec 'taskkill /f /im launcher.exe'
  nsExec::Exec 'taskkill /f /im core.exe'
  nsExec::Exec 'taskkill /f /im worker.exe'
  Delete "$DESKTOP\壹课管家.lnk"
  Delete "$SMPROGRAMS\OneClassMesh\壹课管家.lnk"
  RMDir "$SMPROGRAMS\OneClassMesh"
  SetRegView 64
  DeleteRegValue HKLM "${REG_RUN}" "OneClassMesh"
  DeleteRegKey HKLM "${REG_UNINST}"
  SetRegView lastused
  ; safe removal: only the three component folders this installer owns, plus
  ; the uninstaller. NEVER blanket-delete $INSTDIR (a wrong/user-chosen root
  ; may hold unrelated folders).
  RMDir /r "$INSTDIR\壹课管家"
  RMDir /r "$INSTDIR\bin"
  RMDir /r "$INSTDIR\conf"
  Delete "$INSTDIR\Uninstall 壹课管家.exe"
  RMDir "$INSTDIR"
SectionEnd
