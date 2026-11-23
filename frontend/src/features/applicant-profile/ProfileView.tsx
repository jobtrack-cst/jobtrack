import { Box, Stack, Typography } from '@mui/material';
import { TagChip } from '../../components/JobCard';
import { tokens } from '../../theme';
import type { ApplicantProfile, Education } from './api';

export interface EducationRow { institution: string; qualification: string; startYear: string; endYear: string }
export interface ProfileValues { summary: string; rows: EducationRow[]; skills: string[] }

export const toRow = (e: Education): EducationRow => ({ institution: e.institution, qualification: e.qualification, startYear: String(e.startYear), endYear: e.endYear ? String(e.endYear) : '' });
export const profileValues = (profile: ApplicantProfile): ProfileValues =>
  ({ summary: profile.summary ?? '', rows: (profile.education ?? []).map(toRow), skills: profile.skills ?? [] });

// Read-only view of an applicant profile: the applicant's own saved profile (JT-109) and
// the profile the owning employer sees for an application (JT-111). `heading` is h3 when
// the view sits inside a section that has its own h2.
export function ProfileView({ saved, heading = 'h2' }: { saved: ProfileValues; heading?: 'h2' | 'h3' }) {
  return <Stack spacing={3}>
    <Box>
      <Typography variant={heading} component={heading} sx={{ mb: 1 }}>Summary</Typography>
      <Typography color="text.secondary" sx={{ whiteSpace: 'pre-line' }}>{saved.summary.trim() || 'No summary.'}</Typography>
    </Box>
    <Box>
      <Typography variant={heading} component={heading} sx={{ mb: 1 }}>Education</Typography>
      {saved.rows.length === 0 ? <Typography color="text.secondary">No education entries.</Typography>
        : <Stack component="ul" spacing={1} sx={{ m: 0, pl: 3, color: tokens.textSecondary }}>
          {saved.rows.map((row, index) => <Typography component="li" key={index}>
            <Box component="strong" sx={{ color: tokens.text }}>{row.institution}</Box>, {row.qualification}, {row.startYear} to {row.endYear || 'ongoing'}
          </Typography>)}
        </Stack>}
    </Box>
    <Box>
      <Typography variant={heading} component={heading} sx={{ mb: 1 }}>Skills</Typography>
      {saved.skills.length === 0 ? <Typography color="text.secondary">No skills.</Typography>
        : <Stack direction="row" spacing={1} useFlexGap sx={{ flexWrap: 'wrap' }}>
          {saved.skills.map(skill => <TagChip key={skill} label={skill} />)}
        </Stack>}
    </Box>
  </Stack>;
}
