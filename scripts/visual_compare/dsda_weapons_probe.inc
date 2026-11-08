#include "p_pspr.h"
extern int weapon_preferences[2][NUMWEAPONS+1];
static void GD_WeaponsProbe(void)
{
 const char *path=getenv("GD_DSDA_WEAPON_CASES"),*output=getenv("GD_DSDA_WEAPON_OUT");FILE *in,*out;char name[128],prefs[10];
 int boom,automatic,ready,owned,bullets,shells,cells,rockets,pattern,slot,tics,insurance,strength;unsigned seed;
 if(!path)return;in=fopen(path,"r");out=output?fopen(output,"w"):NULL;if(!in||!out)I_Error("weapon probe files");
 fprintf(out,"name,tic,buttons,ready,pending,attackdown,refire,bullets,shells,cells,rockets,state,tics,y,flash,ftics,fallback,playindex,seeds\n");
 while(fscanf(in,"%127s %d %d %d %d %d %d %d %d %d %d %d %u %d %9s %d",name,&boom,&automatic,&ready,&owned,&bullets,&shells,&cells,&rockets,&pattern,&slot,&tics,&seed,&insurance,prefs,&strength)==16){
  player_t *p;int i,tic,done=0;
  compatibility_level=boom?boom_202_compatibility:doom2_19_compatibility;demo_insurance=insurance;rngseed=seed;gametic=boom_basetic=0;nomonsters=true;weapon_recoil=0;player_bobbing=1;G_InitNew(2,1,1,true);menuactive=false;p=&players[0];
  for(i=0;i<NUMWEAPONS;++i)p->weaponowned[i]=(owned>>i)&1;
  p->ammo[am_clip]=bullets;p->ammo[am_shell]=shells;p->ammo[am_cell]=cells;p->ammo[am_misl]=rockets;
  p->powers[pw_strength]=strength;
  p->readyweapon=ready;p->attackdown=p->refire=0;P_SetupPsprites(p);
  for(i=0;i<10;++i)weapon_preferences[0][i]=weapon_preferences[1][i]=i<9?prefs[i]-'0':0;
  for(tic=0;tic<tics;++tic){
   int fire=pattern==0||pattern==1&&tic<30||pattern==2&&(tic<30||tic>=80)||pattern==3&&(tic<55||tic>=110);int buttons=fire?BT_ATTACK:0;
   if(pattern==3&&tic==40)buttons|=BT_CHANGE|slot<<BT_WEAPONSHIFT;
   if(automatic&&boom){
    if(!p->attackdown)done=0;
    if(p->attackdown&&!P_CheckAmmo(p)&&(!done||(fire&&p->pendingweapon==wp_nochange))){done=1;buttons= (buttons&~BT_WEAPONMASK)|BT_CHANGE|(P_SwitchWeapon(p)<<BT_WEAPONSHIFT);}
    else if((pattern==4||pattern==5)&&(tic==40||tic==100||tic==160)){
     int next=slot;
     if(next==wp_fist&&p->weaponowned[wp_chainsaw]&&p->readyweapon!=wp_chainsaw&&(p->readyweapon==wp_fist||!p->powers[pw_strength]||P_WeaponPreferred(wp_chainsaw,wp_fist)))next=wp_chainsaw;
     if(next==wp_shotgun&&p->weaponowned[wp_supershotgun]&&(!p->weaponowned[wp_shotgun]||p->readyweapon==wp_shotgun||(p->readyweapon!=wp_supershotgun&&P_WeaponPreferred(wp_supershotgun,wp_shotgun))))next=wp_supershotgun;
     buttons|=BT_CHANGE|next<<BT_WEAPONSHIFT;
    }
   }
   memset(&p->cmd,0,sizeof(p->cmd));p->cmd.buttons=buttons;gametic=tic;P_Ticker();
   fprintf(out,"%s,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%ld,%d,%d,%ld,%d,%d,%d,",name,tic,buttons,p->readyweapon,p->pendingweapon,!!p->attackdown,!!p->refire,p->ammo[am_clip],p->ammo[am_shell],p->ammo[am_cell],p->ammo[am_misl],p->psprites[ps_weapon].state?(long)(p->psprites[ps_weapon].state-states):0,p->psprites[ps_weapon].tics,p->psprites[ps_weapon].sy,p->psprites[ps_flash].state?(long)(p->psprites[ps_flash].state-states):0,p->psprites[ps_flash].tics,P_SwitchWeapon(p),rng.rndindex);
   if(boom)for(i=0;i<NUMPRCLASS;++i)if(i!=pr_misc)fprintf(out,"%u;",rng.seed[i]);fputc('\n',out);
  }
 }
 if(!feof(in))I_Error("bad weapon manifest");fclose(in);fclose(out);exit(0);
}
