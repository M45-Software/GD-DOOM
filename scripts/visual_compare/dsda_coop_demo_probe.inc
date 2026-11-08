/* Read-only observation after real multiplayer G_Ticker commands. */
extern void GD_FinaleFields(FILE *,int);
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
static void GD_CoopDemoFrame(void)
{
 GD_CoopSummary();
 const char *path=getenv("GD_DSDA_COOP_OUT");
 static FILE *out;static int last=-1,first=-1;
 int campaign=getenv("GD_COOP_CAMPAIGN")!=NULL;
 if(first<0)first=gametic;
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
  fputc('\n',out);fflush(out);return;
 }
 if(!path||!demoplayback||gamestate!=GS_LEVEL||leveltime<=0||gametic==last)return;
 last=gametic;
 if(!out){out=fopen(path,"w");if(!out)I_Error("co-op output");if(campaign)fprintf(out,"demotic,map,");fprintf(out,"tic,players,bodies,items,sectors,playindex,seeds\n");}
 if(campaign)fprintf(out,"%d,MAP%02d,",gametic-first+1,gamemap);
 fprintf(out,"%d,",leveltime-1);
 for(slot=0;slot<4;++slot)if(playeringame[slot]){
  player_t *p=&players[slot];mobj_t *m=p->mo;int cards=0;
  for(i=0;i<NUMCARDS;++i)if(p->cards[i])cards|=1<<i;
  fprintf(out,"%d:%d:%d:%d:%u:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%ld:%d:%d:%d:%d",slot+1,m->x,m->y,m->z,m->angle,m->momx,m->momy,m->momz,p->health,m->health,p->armorpoints[0],p->ammo[am_clip],p->ammo[am_shell],p->ammo[am_cell],p->ammo[am_misl],p->playerstate,p->viewheight,p->viewz,p->readyweapon,p->pendingweapon,p->psprites[ps_weapon].state?(long)(p->psprites[ps_weapon].state-states):0,p->psprites[ps_weapon].tics,!!p->attackdown,!!p->refire,cards);
  fprintf(out,":%d:%d",p->attacker?p->attacker->x:0,p->attacker?p->attacker->y:0);fputc(';',out);
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
 fprintf(out,",%d,",rng.rndindex);for(i=0;i<NUMPRCLASS;++i)if(i!=pr_misc)fprintf(out,"%u;",rng.seed[i]);
 fputc('\n',out);fflush(out);
}
