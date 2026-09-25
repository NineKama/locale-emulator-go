#include <windows.h>
#include <stdio.h>
#include <string.h>
#include <wchar.h>

int main(void){
 char path[32768],cwd[32768],full[32768],*part=NULL,tiny[5];
 WCHAR expected[32768],actual[32768],expectedCwd[32768];
 DWORD n=GetModuleFileNameA(NULL,path,sizeof(path));
 GetModuleFileNameW(NULL,expected,32768);
 int pass=n>0 && n<sizeof(path) && strlen(path)==n;
 MultiByteToWideChar(932,0,path,-1,actual,32768);
 pass=pass && wcscmp(actual,expected)==0;
 HANDLE file=CreateFileA(path,GENERIC_READ,FILE_SHARE_READ,NULL,OPEN_EXISTING,0,NULL);
 char magic[2]={0};DWORD read=0;
 if(file!=INVALID_HANDLE_VALUE){ReadFile(file,magic,2,&read,NULL);CloseHandle(file);}
 pass=pass && read==2 && magic[0]=='M' && magic[1]=='Z';
 pass=pass && GetFileAttributesA(path)!=INVALID_FILE_ATTRIBUTES;
 DWORD req=GetCurrentDirectoryA(0,NULL);
 DWORD c=GetCurrentDirectoryA(sizeof(cwd),cwd);
 GetCurrentDirectoryW(32768,expectedCwd);
 MultiByteToWideChar(932,0,cwd,-1,actual,32768);
 pass=pass && req==c+1 && wcscmp(actual,expectedCwd)==0;
 DWORD fn=GetFullPathNameA(path,sizeof(full),full,&part);
 pass=pass && fn==strlen(full) && strcmp(full,path)==0 && part && part>=full && part<full+fn;
 MultiByteToWideChar(932,0,part?part:"",-1,actual,32768);
 pass=pass && wcscmp(actual,wcsrchr(expected,L'\\')+1)==0;
 SetLastError(0);
 DWORD small=GetModuleFileNameA(NULL,tiny,sizeof(tiny));DWORD smallError=GetLastError();
 pass=pass && small==sizeof(tiny) && smallError==ERROR_INSUFFICIENT_BUFFER && tiny[4]==0;
 char *cmd=GetCommandLineA();
 if(cmd){MultiByteToWideChar(932,0,cmd,-1,actual,32768);pass=pass && wcscmp(actual,GetCommandLineW())==0 && cmd==GetCommandLineA();}else{pass=0;}
 for(int i=0;i<300;i++){
  HANDLE h=CreateFileA(path,GENERIC_READ,FILE_SHARE_READ,NULL,OPEN_EXISTING,0,NULL);
  if(h==INVALID_HANDLE_VALUE){pass=0;break;}CloseHandle(h);
 }
 SetLastError(0);
 file=CreateFileA("__locale_missing_file__.tmp",GENERIC_READ,FILE_SHARE_READ,NULL,OPEN_EXISTING,0,NULL);
 DWORD missing=GetLastError();
 pass=pass && file==INVALID_HANDLE_VALUE && missing==ERROR_FILE_NOT_FOUND;
 FILE *out=fopen("path-result.txt","w");if(!out)return 2;
 fprintf(out,"%s bits=%u module=%lu file=%c%c cwd=%lu smallError=%lu missing=%lu\n",pass?"PASS":"FAIL",(unsigned)(sizeof(void*)*8),n,magic[0],magic[1],c,smallError,missing);
 fclose(out);return pass?0:1;
}
