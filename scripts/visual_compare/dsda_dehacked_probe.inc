/* Included at the end of d_deh.c in an isolated reference build. It observes
 * resolved tables after PostProcessDeh and exits before gameplay or rendering.
 * No parser, table, action or gameplay behavior is changed. */
#include "p_inter.h"

static void gd_deh_quote(FILE *f, const char *s)
{
  const unsigned char *p = (const unsigned char *)(s ? s : "");
  fputc('"', f);
  for (; *p; ++p) {
    if (*p == '"' || *p == '\\') { fputc('\\', f); fputc(*p, f); }
    else if (*p < 32 || *p >= 127) fprintf(f, "\\u%04x", *p);
    else fputc(*p, f);
  }
  fputc('"', f);
}

void GD_DehackedProbe(void)
{
  const char *path = getenv("GD_DEHACKED_TABLES");
  FILE *f;
  int i, j;
  extern byte *edited_mobjinfo_bits;
  if (!path || !*path) return;
  f = fopen(path, "wb");
  if (!f) { perror(path); exit(2); }
  for (i = 0; i < 137; ++i) {
    mobjinfo_t *m = dsda_GetDehMobjInfo(i).info;
    unsigned int bits = 0;
    for (j = 0; j < 32; ++j)
      if (m->flags & deh_mobjflags_standard[j].value) bits |= 1u << j;
    fprintf(f,"[\"thing\",%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%u,%d]\n",
      i,m->doomednum,m->spawnstate,m->spawnhealth,m->seestate,m->seesound,m->reactiontime,
      m->attacksound,m->painstate,m->painchance,m->painsound,m->meleestate,m->missilestate,
      m->deathstate,m->xdeathstate,m->deathsound,m->speed,m->radius,m->height,m->mass,
      m->damage,m->activesound,bits,m->raisestate);
    fprintf(f,"[\"defined-bits\",%d,%d]\n",i,edited_mobjinfo_bits[i]?1:0);
  }
  for (i = 0; i < 967; ++i) {
    state_t *s = dsda_GetDehState(i).state;
    const char *name = s->action ? "UNKNOWN" : "NULL";
    if (s->action)
      for (j = 0; j < sizeof(deh_bexptrs)/sizeof(*deh_bexptrs); ++j)
        if (deh_bexptrs[j].cptr == s->action) { name = deh_bexptrs[j].lookup + 2; break; }
    fprintf(f,"[\"state\",%d,%d,%ld,%ld,",i,s->sprite,s->frame,s->tics);
    gd_deh_quote(f,name);
    fprintf(f,",%d,%ld,%ld]\n",s->nextstate,s->misc1,s->misc2);
  }
  for (i = 0; i < 9; ++i) {
    weaponinfo_t *w = &weaponinfo[i];
    /* DSDA shares the ammo enum with Raven games; DEH's no-ammo value is 5. */
    fprintf(f,"[\"weapon\",%d,%d,%d,%d,%d,%d,%d]\n",i,w->ammo==am_noammo?5:w->ammo,w->upstate,w->downstate,w->readystate,w->atkstate,w->flashstate);
  }
  for (i = 0; i < 4; ++i) fprintf(f,"[\"ammo\",%d,%d,%d]\n",i,maxammo[i],clipammo[i]);
  for (i = 0; i < 109; ++i) {
    sfxinfo_t *s = dsda_GetDehSFX(i);
    const char *name = s->name;
    if (name && !strncasecmp(name,"ds",2)) name += 2;
    fprintf(f,"[\"sound\",%d,",i);gd_deh_quote(f,name);fprintf(f,",%d,%d]\n",s->priority,s->pitch);
  }
  for (i = 0; i < 138; ++i) { fprintf(f,"[\"sprite\",%d,",i);gd_deh_quote(f,sprnames[i]);fprintf(f,"]\n"); }
  for (i = 1; i < 68; ++i) {
    const char *name = S_music[i].name;
    fprintf(f,"[\"music\",%d,",i);gd_deh_quote(f,name);fprintf(f,"]\n");
  }
  for (i = 0; i < deh_numstrlookup; ++i) {
    fprintf(f,"[\"string\",");gd_deh_quote(f,deh_strlookup[i].lookup);fputc(',',f);
    gd_deh_quote(f,*deh_strlookup[i].ppstr);fprintf(f,"]\n");
  }
  for (i = 0; cheat[i].cheat; ++i) {
    if (!cheat[i].deh_cheat) continue;
    for (j = 0; j < i; ++j)
      if (cheat[j].deh_cheat && !strcasecmp(cheat[j].deh_cheat,cheat[i].deh_cheat)) break;
    if (j < i) continue;
    fprintf(f,"[\"cheat\",");gd_deh_quote(f,cheat[i].deh_cheat);fputc(',',f);
    gd_deh_quote(f,cheat[i].cheat);fprintf(f,"]\n");
  }
  for (i=1;i<=3;++i)for(j=1;j<=9;++j)fprintf(f,"[\"par\",\"E%dM%d\",%d]\n",i,j,pars[i][j]);
  for (i=1;i<=32;++i)fprintf(f,"[\"par\",\"MAP%02d\",%d]\n",i,cpars[i-1]);
#define GD_MISC(label, value) fprintf(f,"[\"misc\",\"" label "\",%d]\n", value)
  GD_MISC("InitialHealth",initial_health); GD_MISC("InitialBullets",initial_bullets);
  /* Export the parsed definitions, before compatibility derives gameplay limits. */
  GD_MISC("MaxHealth",IsDehMaxHealth?deh_maxhealth:100); GD_MISC("MaxArmor",max_armor);
  GD_MISC("GreenArmorClass",green_armor_class); GD_MISC("BlueArmorClass",blue_armor_class);
  GD_MISC("MaxSoulsphere",IsDehMaxSoul?deh_max_soul:200); GD_MISC("SoulsphereHealth",soul_health);
  GD_MISC("MegasphereHealth",IsDehMegaHealth?deh_mega_health:200); GD_MISC("GodModeHealth",god_health);
  GD_MISC("IDFAArmor",idfa_armor); GD_MISC("IDFAArmorClass",idfa_armor_class);
  GD_MISC("IDKFAArmor",idkfa_armor); GD_MISC("IDKFAArmorClass",idkfa_armor_class);
  GD_MISC("BFGCellsPerShot",bfgcells); GD_MISC("MonstersInfight",monsters_infight?221:202);
#undef GD_MISC
  fprintf(f,"[\"defined-misc\",\"MaxHealth\",%d]\n",IsDehMaxHealth?1:0);
  fprintf(f,"[\"defined-misc\",\"MaxSoulsphere\",%d]\n",IsDehMaxSoul?1:0);
  fprintf(f,"[\"defined-misc\",\"MegasphereHealth\",%d]\n",IsDehMegaHealth?1:0);
  fprintf(f,"[\"defined-misc\",\"BFGCellsPerShot\",%d]\n",bfgcells_modified?1:0);
  if (fclose(f)) { perror(path); exit(2); }
  exit(0);
}
