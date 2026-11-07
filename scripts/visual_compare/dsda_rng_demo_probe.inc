/* Observe completed real G_Ticker demo tics. Gameplay code is unchanged. */
static void GD_RNGDemoFrame(void)
{
 static FILE *out;static int last_gametic=-1,summary_written,capture_tic,previous_leveltime,reloads,first_gametic=-1;
 const char *path=getenv("GD_DSDA_RNG_DEMO_OUT");thinker_t *th=NULL;int i;
 if(!path||!demoplayback)return;
 if(out&&!summary_written&&gamestate==GS_INTERMISSION&&getenv("GD_RNG_FULL_MAP")) {
  const char *summary_path=getenv("GD_RNG_SUMMARY");FILE *summary=summary_path?fopen(summary_path,"w"):NULL;player_t *p=&players[displayplayer];
  if(!summary)I_Error("full combat summary output");
  fprintf(summary,"{\"completed\":true,\"leveltime\":%d,\"kills\":%d,\"total_kills\":%d,\"secrets\":%d,\"total_secrets\":%d,\"health\":%d}\n",leveltime,p->killcount,totalkills,p->secretcount,totalsecret,p->health);
  fclose(summary);summary_written=1;
 }
 if(gamestate!=GS_LEVEL||leveltime<=0||last_gametic==gametic)return;
 last_gametic=gametic;
 if(first_gametic<0)first_gametic=gametic;
 if(getenv("GD_RNG_REBIRTH")&&leveltime<previous_leveltime)reloads++;
 previous_leveltime=leveltime;
 /* Controlled initial corpse, not a replacement resurrection implementation. */
 if(leveltime==1 && getenv("GD_RNG_CRUSHED_CORPSE")) {
  thinker_t *it=NULL;
  while((it=P_NextThinker(it,th_all)))if(it->function==P_MobjThinker){
   mobj_t *corpse=(mobj_t*)it;
   if(corpse->type==MT_SHOTGUY){
    P_DamageMobj(corpse,NULL,players[displayplayer].mo,200);
    P_SetMobjState(corpse,S_GIBS);corpse->flags&=~MF_SOLID;
    corpse->height=corpse->radius=0;
    P_MapEnd();
   }
  }
 }
 if(!out){out=fopen(path,"w");if(!out)I_Error("RNG demo output");fprintf(out,"tic,x,y,z,angle,momx,momy,momz,health,armor,ammo,viewz,playindex,mobjs,seeds");
  if(getenv("GD_RNG_FULL_MAP")||getenv("GD_RNG_REBIRTH"))fprintf(out,",shells,cells,rockets,ready,pending,weaponstate,weapontics,flash,flashtics,attackdown,refire,kills,secrets,sectors");
  if(getenv("GD_RNG_REBIRTH")) {
   fprintf(out,",leveltime,playerstate,viewheight,deltaviewheight,reloads,logictic");
  }
  if(getenv("GD_RNG_CAMPAIGN"))fprintf(out,",demotic,map,cards");
  fputc('\n',out);
 }
 {player_t *p=&players[displayplayer];mobj_t *m=p->mo;
 fprintf(out,"%d,%d,%d,%d,%u,%d,%d,%d,%d,%d,%d,%d,%d,",getenv("GD_RNG_REBIRTH")?capture_tic++:leveltime-1,m->x,m->y,m->z,m->angle,m->momx,m->momy,m->momz,p->health,p->armorpoints[0],p->ammo[am_clip],p->viewz,rng.rndindex);}
 while((th=P_NextThinker(th,th_all)))if(th->function==P_MobjThinker){mobj_t *m=(mobj_t*)th;if(m->player || (m->flags&MF_SPECIAL))continue;
 if(getenv("GD_RNG_COMBAT_ONLY") && !(getenv("GD_RNG_FULL_MAP") && m->type==MT_BARREL) && !(m->flags&(MF_COUNTKILL|MF_MISSILE)) && m->type!=MT_TRACER && m->type!=MT_FATSHOT && m->type!=MT_BRUISERSHOT && m->type!=MT_TROOPSHOT && m->type!=MT_HEADSHOT && m->type!=MT_ROCKET && m->type!=MT_PLASMA && m->type!=MT_BFG && m->type!=MT_ARACHPLAZ && m->type!=MT_EXTRABFG && m->type!=MT_SKULL && m->type!=MT_PUFF && m->type!=MT_BLOOD && m->type!=MT_SMOKE && m->type!=MT_FIRE && m->type!=MT_TFOG && m->type!=MT_IFOG)continue;
  fprintf(out,"%d:%d:%d:%d:%u:%d:%d:%d:%d:%d:%ld",m->type,m->x,m->y,m->z,m->angle,m->momx,m->momy,m->momz,m->health,m->tics,(long)(m->state-states));
  if(getenv("GD_RNG_COMBAT_ONLY"))fprintf(out,":%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d",m->target!=NULL,m->target?m->target->type:0,m->tracer!=NULL,m->tracer?m->tracer->type:0,m->movedir,m->movecount,m->reactiontime,m->threshold,m->lastlook,m->radius,m->height,m->floorz,m->ceilingz);
  fputc(';',out);}
 fputc(',',out);for(i=0;i<NUMPRCLASS;++i)if(i!=pr_misc)fprintf(out,"%u;",rng.seed[i]);
 if(getenv("GD_RNG_FULL_MAP")||getenv("GD_RNG_REBIRTH")) {player_t *p=&players[displayplayer];
  fprintf(out,",%d,%d,%d,%d,%d,%ld,%d,%ld,%d,%d,%d,%d,%d,",p->ammo[am_shell],p->ammo[am_cell],p->ammo[am_misl],p->readyweapon,p->pendingweapon,
   p->psprites[ps_weapon].state?(long)(p->psprites[ps_weapon].state-states):0,p->psprites[ps_weapon].tics,
   p->psprites[ps_flash].state?(long)(p->psprites[ps_flash].state-states):0,p->psprites[ps_flash].tics,!!p->attackdown,!!p->refire,p->killcount,p->secretcount);
  for(i=0;i<numsectors;++i)fprintf(out,"%d:%d:%d;",sectors[i].floorheight,sectors[i].ceilingheight,sectors[i].lightlevel);
 }
 /* Export the completed-tic phase; boom_logictic is still the current tic
    until the outer main loop advances gametic. */
 if(getenv("GD_RNG_REBIRTH")){player_t *p=&players[displayplayer];fprintf(out,",%d,%d,%d,%d,%d,%u",leveltime,p->playerstate,p->viewheight,p->deltaviewheight,reloads,boom_logictic+1);}
 if(getenv("GD_RNG_CAMPAIGN")){int cards=0;for(i=0;i<NUMCARDS;++i)if(players[displayplayer].cards[i])cards|=1<<i;fprintf(out,",%d,MAP%02d,%d",gametic-first_gametic,gamemap,cards);}
 fputc('\n',out);fflush(out);
}
