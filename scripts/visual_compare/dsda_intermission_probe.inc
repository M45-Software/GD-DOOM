/* Read-only observation of real WI_Ticker counters. */
void GD_IntermissionFields(FILE *out)
{
 int i;
 if(gamestate!=GS_INTERMISSION){fprintf(out,",0,0,0,0,0,0,,0,0");return;}
 fprintf(out,",%d,%d,%d,%d,%d,%d,",state,bcnt,cnt,netgame?ng_state:sp_state,cnt_pause,!!acceleratestage);
 for(i=0;i<(netgame?4:1);i++)if(!netgame||playeringame[i])fprintf(out,"%d:%d:%d:%d;",cnt_kills[i],cnt_items[i],cnt_secret[i],netgame?cnt_frags[i]:0);
 fprintf(out,",%d,%d",netgame?0:cnt_time,netgame?0:cnt_par);
}
