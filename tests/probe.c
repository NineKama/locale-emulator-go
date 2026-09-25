// Independent native fixture: verifies actual Win32 imports, not Go wrappers.
#include <windows.h>
#include <stdio.h>

int main(void) {
 WCHAR decoded[4]={0}; char encoded[8]={0};
 const char sjis[]={ (char)0x82,(char)0xa0,0 }; // Hiragana A
 const WCHAR wide[]={0x3042,0};
 int n=MultiByteToWideChar(CP_ACP,0,sjis,2,decoded,4);
 int m=WideCharToMultiByte(CP_ACP,0,wide,1,encoded,8,NULL,NULL);
 WCHAR explicitUtf8[4]={0};
 const char utf8[]={ (char)0xe3,(char)0x81,(char)0x82 };
 int u=MultiByteToWideChar(CP_UTF8,0,utf8,3,explicitUtf8,4);
 SetLastError(0);
 int bad=MultiByteToWideChar(99999,0,"a",1,decoded+2,1);
 DWORD err=GetLastError();
 WCHAR oem[4]={0},thread[4]={0};
 int o=MultiByteToWideChar(CP_OEMCP,0,sjis,2,oem,4);
 int t=MultiByteToWideChar(CP_THREAD_ACP,0,sjis,2,thread,4);
 SetLastError(1234); volatile UINT cp=GetACP(); DWORD preserved=GetLastError();
 int pass=GetACP()==932 && GetOEMCP()==932 && GetUserDefaultLCID()==0x411 && GetSystemDefaultLCID()==0x411 && GetThreadLocale()==0x411 && n==1 && decoded[0]==0x3042 && m==2 && (unsigned char)encoded[0]==0x82 && (unsigned char)encoded[1]==0xa0 && u==1 && explicitUtf8[0]==0x3042 && bad==0 && err==ERROR_INVALID_PARAMETER;
 pass=pass && o==1 && oem[0]==0x3042 && t==1 && thread[0]==0x3042 && cp==932 && preserved==1234;
 // Repeated Win32 calls catch x86 argument/stack-cleanup regressions.
 for(int i=0;i<10000;i++) {
  WCHAR w[2]={0};char b[4]={0};
  if(MultiByteToWideChar(CP_ACP,0,sjis,2,w,2)!=1 || w[0]!=0x3042 ||
     WideCharToMultiByte(CP_ACP,0,w,1,b,4,NULL,NULL)!=2 ||
     (unsigned char)b[0]!=0x82 || (unsigned char)b[1]!=0xa0) {pass=0;break;}
 }
 FILE *f=fopen("probe-result.txt","w"); if(!f)return 2;
 fprintf(f,"%s bits=%u ACP=%u OEM=%u LCID=%lu decode=%04x encode=%02x%02x utf8=%04x error=%lu\n",pass?"PASS":"FAIL",(unsigned)(sizeof(void*)*8),GetACP(),GetOEMCP(),GetUserDefaultLCID(),decoded[0],(unsigned char)encoded[0],(unsigned char)encoded[1],explicitUtf8[0],err);
 fclose(f);return pass?0:1;
}
