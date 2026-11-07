/* Controlled API experiment, not a netdemo synchronization claim. Both
 * engines receive the same explicit body damage and player-rebirth calls. */
static void GD_VoodooLifecycle(void)
{
 const char *path=getenv("GD_DSDA_VOODOO_OUT");
 static int done;
 thinker_t *it=NULL;
 mobj_t *dolls[2]={NULL,NULL};
 FILE *out;
 int count=0,event,slot;
 if(!path||done||!demoplayback||gamestate!=GS_LEVEL||leveltime!=1)return;
 done=1;
 while((it=P_NextThinker(it,th_all)))if(it->function==P_MobjThinker){
  mobj_t *m=(mobj_t*)it;
  if(m->player==&players[1] && m!=players[1].mo && count<2)dolls[count++]=m;
 }
 if(count!=2)I_Error("voodoo fixture needs two slot-2 dolls");
 out=fopen(path,"w");if(!out)I_Error("voodoo output");
 fprintf(out,"event,slot,health,bodyhealth,dead,dolls\n");
 for(event=0;event<7;++event){
  switch(event){
   case 1:P_DamageMobj(dolls[0],NULL,NULL,30);break;
   case 2:P_DamageMobj(dolls[1],NULL,NULL,20);break;
   case 3:P_DamageMobj(dolls[1],NULL,NULL,50);break;
   case 4:P_DamageMobj(dolls[0],NULL,NULL,70);break;
   case 5:G_PlayerReborn(1);P_SpawnPlayer(1,&playerstarts[0][1]);break;
   case 6:P_DamageMobj(dolls[1],NULL,NULL,30);break;
  }
  P_MapEnd();
  for(slot=0;slot<2;++slot){
   int i;player_t *p=&players[slot];
   fprintf(out,"%d,%d,%d,%d,%d,",event,slot+1,p->health,p->mo->health,p->playerstate==PST_DEAD);
   for(i=0;i<2;++i){mobj_t *m=dolls[i];fprintf(out,"%d:%d:%d:%d:%d;",m->health,!!(m->flags&MF_CORPSE),m->height,!!(m->flags&MF_SOLID),!!(m->flags&MF_SHOOTABLE));}
   fputc('\n',out);
  }
 }
 fclose(out);
}
