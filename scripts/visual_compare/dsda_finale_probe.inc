/* Finale observation after a completed real G_Ticker. Optional key events are
 * explicit host input for the cast experiment, never replacement ticker logic. */
void GD_FinaleFields(FILE *out, int command)
{
 const char *keys=getenv("GD_COOP_CAST_KEYS");
 if(gamestate==GS_FINALE && finalestage==2 && keys){
  const char *p=keys;while(*p){char *end;long tic=strtol(p,&end,10);if(end==p)break;
   if(tic==command){event_t ev;memset(&ev,0,sizeof(ev));ev.type=ev_keydown;F_Responder(&ev);}
   p=(*end==',')?end+1:end;
  }
 }
 if(gamestate!=GS_FINALE){fprintf(out,"0,0,0,0,0,0,0,0,0,0,");return;}
 fprintf(out,"%d,%d,%d,",finalecount,finalestage,!!midstage);
 if(finalestage==2)fprintf(out,"%d,%ld,%d,%d,%d,%d,%d,",castnum,(long)(caststate-states),casttics,castframes,castonmelee,!!castattacking,!!castdeath);
 else fprintf(out,"0,0,0,0,0,0,0,");
}
