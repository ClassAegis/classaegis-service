; OneClassMesh agent NSIS include (YikeGuanjia).
; electron-builder includes this file via nsis.include.

!macro customInstall
  ; Desktop shortcut -> launcher.exe -with-ui. Launcher guards core/worker and
  ; opens the display app. Icon borrows the Electron exe (Go exes have no icon).
  CreateShortcut "$DESKTOP\壹课管家.lnk" "$INSTDIR\resources\ocm\bin\launcher.exe" "-with-ui" "$INSTDIR\壹课管家.exe"

  ; Boot autostart at user logon for classroom machines.
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "OneClassMesh" '"$INSTDIR\resources\ocm\bin\launcher.exe" -with-ui'
!macroend

!macro customUnInstall
  ; Stop agent processes so uninstall does not hit locked files.
  nsExec::Exec 'taskkill /f /im 壹课管家.exe'
  nsExec::Exec 'taskkill /f /im launcher.exe'
  nsExec::Exec 'taskkill /f /im core.exe'
  nsExec::Exec 'taskkill /f /im worker.exe'
  Delete "$DESKTOP\壹课管家.lnk"
  DeleteRegValue HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "OneClassMesh"
  RMDir /r "$INSTDIR\resources\ocm"
!macroend
