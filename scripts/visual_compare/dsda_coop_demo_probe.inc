/* Read-only observation after real multiplayer G_Ticker commands. */
extern void GD_FinaleFields(FILE *,int);
// player->attacker is not reference-counted. Track removal and assignment
// independently so allocator reuse cannot turn a stale pointer into a new
// attacker. These callbacks observe events without changing gameplay state.
static int GD_RemovedAttacker[MAX_MAXPLAYERS];
void GD_CoopAttackerAssigned(player_t *player)
{
 GD_RemovedAttacker[player-players]=0;
}
void GD_CoopAttackerRemoved(mobj_t *actor)
{
 int slot;
 for(slot=0;slot<MAX_MAXPLAYERS;slot++)if(!actor||players[slot].attacker==actor)GD_RemovedAttacker[slot]=1;
}
static mobj_t *GD_LiveAttacker(int slot)
{
 mobj_t *attacker=players[slot].attacker;
 thinker_t *it=NULL;
 if(GD_RemovedAttacker[slot])return NULL;
 if(attacker)while((it=P_NextThinker(it,th_all)))if(it->function==P_MobjThinker && (mobj_t*)it==attacker)return attacker;
 return NULL;
}
static void GD_CoopSummary(void)
{
 static int done;const char *path=getenv("GD_COOP_SUMMARY");FILE *out;int slot,first=1;
 if(done||!path||!demoplayback||gameaction!=ga_completed)return;
 out=fopen(path,"w");if(!out)I_Error("co-op summary");done=1;
 fprintf(out,"{\"completed\":true,\"map\":\"MAP%02d\",\"leveltime\":%d,\"total_kills\":%d,\"total_items\":%d,\"total_secrets\":%d,\"players\":[",gamemap,leveltime,totalkills,totalitems,totalsecret);
 for(slot=0;slot<4;slot++)if(playeringame[slot]){player_t *p=&players[slot];
  if(!first)fputc(',',out);first=0;
  fprintf(out,"{\"slot\":%d,\"health\":%d,\"armor\":%d,\"bullets\":%d,\"shells\":%d,\"cells\":%d,\"rockets\":%d,\"kills\":%d,\"items\":%d,\"secrets\":%d}",slot+1,p->health,p->armorpoints[0],p->ammo[am_clip],p->ammo[am_shell],p->ammo[am_cell],p->ammo[am_misl],p->killcount,p->itemcount,p->secretcount);
 }
 fprintf(out,"]}\n");fclose(out);
}
extern void GD_IntermissionFields(FILE *);
static void GD_CampaignPhaseFrame(int command)
{
 const char *path=getenv("GD_DSDA_PHASE_OUT");static FILE *out;
 if(!path||!demoplayback)return;
 if(!out){out=fopen(path,"w");if(!out)I_Error("campaign phases");fprintf(out,"demotic,map,phase,count,stage,accelerated,castnum,caststate,casttics,castframes,melee,attacking,death,players,playindex,seeds,wi_screen,wi_bcnt,wi_cnt,wi_state,wi_pause,wi_accelerate,wi_counts,wi_time,wi_par\n");}
 int slot,i;
  fprintf(out,"%d,",command);
  if(gamemode==commercial)fprintf(out,"MAP%02d,",gamemap);else fprintf(out,"E%dM%d,",gameepisode,gamemap);
  fprintf(out,"%d,",gamestate==GS_FINALE?2:gamestate==GS_INTERMISSION?1:0);
  GD_FinaleFields(out,command);
  for(slot=0;slot<4;slot++)if(playeringame[slot]){player_t *p=&players[slot];int cards=0;for(i=0;i<NUMCARDS;i++)if(p->cards[i])cards|=1<<i;
   fprintf(out,"%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d;",slot+1,p->health,p->armorpoints[0],p->ammo[am_clip],p->ammo[am_shell],p->ammo[am_cell],p->ammo[am_misl],p->readyweapon,p->pendingweapon,!!p->attackdown,!!p->usedown,cards);
  }
  fprintf(out,",%d,",rng.rndindex);for(i=0;i<NUMPRCLASS;i++)if(i!=pr_misc)fprintf(out,"%u;",rng.seed[i]);
 GD_IntermissionFields(out);fputc('\n',out);fflush(out);
}
#include "md5.h"

static void GD_CoopLineState(FILE *out)
{
 struct MD5Context context;unsigned char bytes[4096],hash[16];size_t used=0;int i;
 MD5Init(&context);
 for(i=0;i<numlines;i++){
  unsigned int special=(unsigned short)lines[i].special;
  bytes[used++]=special&255;bytes[used++]=special>>8;
  if(used==sizeof(bytes)){MD5Update(&context,bytes,used);used=0;}
 }
 if(used)MD5Update(&context,bytes,used);
 MD5Final(hash,&context);fputc(',',out);
 for(i=0;i<16;i++)fprintf(out,"%02x",hash[i]);
}

static void GD_CoopLineDetail(const char *path)
{
 size_t length=strlen(path)+sizeof(".lines.json");char *name=malloc(length);FILE *detail;int i;
 if(!name)I_Error("co-op line detail path");
 snprintf(name,length,"%s.lines.json",path);detail=fopen(name,"w");free(name);
 if(!detail)I_Error("co-op line detail output");
 fputc('[',detail);
 for(i=0;i<numlines;i++)fprintf(detail,"%s%u",i?",":"",(unsigned short)lines[i].special);
 fprintf(detail,"]\n");fclose(detail);
}

/* Optional indexed full rows for long-campaign diagnosis. The compact digest
 * still hashes the same row, and these writes do not alter simulation state. */
static void GD_CoopCanonicalFrame(const char *row,size_t size,int command,int campaign,int line_state)
{
 const char *path=getenv("GD_COOP_CANONICAL_OUT");static FILE *rows,*index;
 const char *from=getenv("GD_COOP_CANONICAL_FROM");long offset;
 if(!path||(from&&command<atoi(from)))return;
 if(!rows){
  size_t length=strlen(path)+sizeof(".index.csv");char *name=malloc(length);
  if(!name)I_Error("canonical index path");
  snprintf(name,length,"%s.index.csv",path);index=fopen(name,"w");free(name);
  rows=fopen(path,"w");if(!rows||!index)I_Error("canonical trace output");
  if(campaign)fprintf(rows,"demotic,map,");
  fprintf(rows,"tic,players,bodies,items,sectors");
  if(line_state)fprintf(rows,",line_md5");
  fprintf(rows,",playindex,seeds\n");fprintf(index,"demotic,offset,length\n");
 }
 offset=ftell(rows);if(offset<0)I_Error("canonical trace offset");
 if(fwrite(row,1,size,rows)!=size)I_Error("canonical trace write");
 fprintf(index,"%d,%ld,%zu\n",command,offset,size);fflush(rows);fflush(index);
}

static void GD_CoopDiagnosticLimit(int command)
{
 /* Explicit diagnostic prefix only; campaign verification leaves this unset. */
 const char *limit=getenv("GD_COOP_CAPTURE_COMMANDS");
 if(limit&&command>=atoi(limit))exit(0);
}

static void GD_CoopDemoFrame(void)
{
 GD_CoopSummary();
 const char *path=getenv("GD_DSDA_COOP_OUT");
 static FILE *out;static int last=-1,first=-1;
 int campaign=getenv("GD_COOP_CAMPAIGN")!=NULL;
 int compact=getenv("GD_COOP_COMPACT")!=NULL;
 int line_state=getenv("GD_COOP_LINES")!=NULL;
 FILE *destination;char *row=NULL;size_t row_size=0;
 if(first<0)first=gametic;
 // Diagnostic output window only: still execute every original command.
 // Official campaign verifiers scrub this environment variable.
 if(getenv("GD_COOP_CAPTURE_FROM_COMMAND") && gametic-first+1<atoi(getenv("GD_COOP_CAPTURE_FROM_COMMAND"))){last=gametic;GD_CoopDiagnosticLimit(gametic-first+1);return;}
 if(gametic!=last)GD_CampaignPhaseFrame(gametic-first+1);
 int slot,i;thinker_t *it=NULL;
 if(path && demoplayback && getenv("GD_COOP_FINALE") && gametic!=last) {
  last=gametic;
  if(!out){out=fopen(path,"w");if(!out)I_Error("finale output");fprintf(out,"demotic,map,phase,count,stage,accelerated,castnum,caststate,casttics,castframes,melee,attacking,death,players,playindex,seeds\n");}
  fprintf(out,"%d,",gametic-first+1);
  if(gamemode==commercial)fprintf(out,"MAP%02d,",gamemap);else fprintf(out,"E%dM%d,",gameepisode,gamemap);
  fprintf(out,"%d,",gamestate==GS_FINALE?2:gamestate==GS_INTERMISSION?1:0);
  GD_FinaleFields(out,gametic-first+1);
  for(slot=0;slot<4;slot++)if(playeringame[slot]){player_t *p=&players[slot];int cards=0;for(i=0;i<NUMCARDS;i++)if(p->cards[i])cards|=1<<i;
   fprintf(out,"%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d;",slot+1,p->health,p->armorpoints[0],p->ammo[am_clip],p->ammo[am_shell],p->ammo[am_cell],p->ammo[am_misl],p->readyweapon,p->pendingweapon,!!p->attackdown,!!p->usedown,cards);
  }
  fprintf(out,",%d,",rng.rndindex);for(i=0;i<NUMPRCLASS;i++)if(i!=pr_misc)fprintf(out,"%u;",rng.seed[i]);
  fputc('\n',out);fflush(out);GD_CoopDiagnosticLimit(gametic-first+1);return;
 }
 if(!path||!demoplayback||gamestate!=GS_LEVEL||leveltime<=0||gametic==last){GD_CoopDiagnosticLimit(gametic-first+1);return;}
 last=gametic;
 if(!out){out=fopen(path,"w");if(!out)I_Error("co-op output");if(campaign)fprintf(out,"demotic,map,");
  if(compact)fprintf(out,"tic,state_md5\n");
  else{fprintf(out,"tic,players,bodies,items,sectors");if(line_state)fprintf(out,",line_md5");fprintf(out,",playindex,seeds\n");}
 }
 destination=out;
 if(compact){out=open_memstream(&row,&row_size);if(!out)I_Error("co-op row buffer");}
 if(campaign)fprintf(out,"%d,MAP%02d,",gametic-first+1,gamemap);
 fprintf(out,"%d,",leveltime-1);
 for(slot=0;slot<4;++slot)if(playeringame[slot]){
  player_t *p=&players[slot];mobj_t *m=p->mo;int cards=0;
  for(i=0;i<NUMCARDS;++i)if(p->cards[i])cards|=1<<i;
  fprintf(out,"%d:%d:%d:%d:%u:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%ld:%d:%d:%d:%d",slot+1,m->x,m->y,m->z,m->angle,m->momx,m->momy,m->momz,p->health,m->health,p->armorpoints[0],p->ammo[am_clip],p->ammo[am_shell],p->ammo[am_cell],p->ammo[am_misl],p->playerstate,p->viewheight,p->viewz,p->readyweapon,p->pendingweapon,p->psprites[ps_weapon].state?(long)(p->psprites[ps_weapon].state-states):0,p->psprites[ps_weapon].tics,!!p->attackdown,!!p->refire,cards);
  fprintf(out,":%d:%d",GD_LiveAttacker(slot)?p->attacker->x:0,GD_LiveAttacker(slot)?p->attacker->y:0);fputc(';',out);
 }
 fputc(',',out);
 while((it=P_NextThinker(it,th_all)))if(it->function==P_MobjThinker){
  mobj_t *m=(mobj_t*)it;
  if((m->player&&m==m->player->mo)||(m->flags&MF_SPECIAL))continue;
  if(getenv("GD_COOP_COMBAT_ONLY")&&(m->type>MT_EXTRABFG||m->type==MT_TELEPORTMAN))continue;
  fprintf(out,"%d:%d:%d:%d:%u:%d:%d:%d:%d:%d:%ld:%d:%d:%d:%d:%d",m->type,m->x,m->y,m->z,m->angle,m->momx,m->momy,m->momz,m->health,m->tics,(long)(m->state-states),m->radius,m->height,m->floorz,m->ceilingz,m->player?(int)(m->player-players)+1:0);
  if(m->flags&MF_COUNTKILL)fprintf(out,":%d:%d:%d:%d:%d:%d:%d:%d:%d:%d",m->target&&m->target->player?(int)(m->target->player-players)+1:0,m->target?m->target->x:0,m->target?m->target->y:0,m->reactiontime,m->threshold,m->movedir,m->movecount,m->lastlook,!!(m->flags&MF_JUSTHIT),!!(m->flags&MF_JUSTATTACKED));
  fputc(';',out);
 }
 fputc(',',out);it=NULL;while((it=P_NextThinker(it,th_all)))if(it->function==P_MobjThinker){mobj_t *m=(mobj_t*)it;if(m->flags&MF_SPECIAL)fprintf(out,"%d:%d:%d:%d;",m->type,m->x,m->y,m->z);}
 fputc(',',out);for(i=0;i<numsectors;++i)fprintf(out,"%d:%d:%d;",sectors[i].floorheight,sectors[i].ceilingheight,sectors[i].lightlevel);
 if(line_state)GD_CoopLineState(out);
 fprintf(out,",%d,",rng.rndindex);for(i=0;i<NUMPRCLASS;++i)if(i!=pr_misc)fprintf(out,"%u;",rng.seed[i]);
 fputc('\n',out);fflush(out);
 if(compact){
  struct MD5Context context;unsigned char hash[16];
  fclose(out);out=destination;
  GD_CoopCanonicalFrame(row,row_size,gametic-first+1,campaign,line_state);
  if(getenv("GD_COOP_DETAIL_COMMAND")&&atoi(getenv("GD_COOP_DETAIL_COMMAND"))==gametic-first+1){
   size_t length=strlen(path)+sizeof(".detail.csv");char *detail_path=malloc(length);FILE *detail;
   if(!detail_path)I_Error("co-op detail path");snprintf(detail_path,length,"%s.detail.csv",path);
   detail=fopen(detail_path,"w");free(detail_path);if(!detail)I_Error("co-op detail output");
   fwrite(row,1,row_size,detail);fclose(detail);
   if(line_state)GD_CoopLineDetail(path);
  }
  MD5Init(&context);MD5Update(&context,(const unsigned char *)row,row_size);MD5Final(hash,&context);free(row);
  if(campaign)fprintf(out,"%d,MAP%02d,",gametic-first+1,gamemap);
  fprintf(out,"%d,",leveltime-1);for(i=0;i<16;i++)fprintf(out,"%02x",hash[i]);fputc('\n',out);fflush(out);
 }
 GD_CoopDiagnosticLimit(gametic-first+1);
}
